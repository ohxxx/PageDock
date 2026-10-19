# [INPUT]: Official WebView2 .NET SDK/loader, controller mode, and an unused evidence directory.
# [OUTPUT]: Standalone controller lifetime/handle measurements, without Go/MyGo/PageDock.
# [POS]: Windows runtime diagnostic; local HTML and one fresh browser profile only.
# [PROTOCOL]: Update tests/soak/AGENTS.md; raw SDK packages/binaries/results belong in .scratch/.
param(
    [Parameter(Mandatory=$true)][string]$SdkDirectory,
    [Parameter(Mandatory=$true)][string]$Output,
    [ValidateRange(1,1000)][int]$Cycles = 60,
    [ValidateRange(1,600)][int]$Recovery = 90,
    [switch]$Composition,
    [switch]$VisualTree,
    [switch]$NavigateOrigins,
    [switch]$Continuous,
    [ValidateRange(0.1,60)][double]$CyclePace = 0.1,
    [ValidateRange(0,60)][double]$Dwell = 0,
    [switch]$RecreateWindow,
    [switch]$ReadPermissions,
    [switch]$EvaluateScript,
    [switch]$RequestFilter,
    [switch]$DetachVisualRoot,
    [string]$TargetVersion = ''
)
$ErrorActionPreference = 'Stop'
if (($VisualTree -or $DetachVisualRoot) -and -not $Composition) { throw 'VisualTree and DetachVisualRoot require Composition' }
if ([Threading.Thread]::CurrentThread.ApartmentState -ne 'STA') { throw 'Run PowerShell with -STA' }
$SdkDirectory = (Resolve-Path -LiteralPath $SdkDirectory).Path
$Output = [IO.Path]::GetFullPath($Output)
if (Test-Path -LiteralPath $Output) { throw 'Evidence directory must be new' }
New-Item -ItemType Directory -Path $Output | Out-Null
$saved = @{}
foreach ($key in @('WEBVIEW2_USER_DATA_FOLDER','WEBVIEW2_BROWSER_EXECUTABLE_FOLDER','WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS')) {
    $saved[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
    [Environment]::SetEnvironmentVariable($key,$null,'Process')
}
try {
    Add-Type -AssemblyName System.Windows.Forms
    Add-Type -AssemblyName System.Drawing
    $core = Join-Path $SdkDirectory 'Microsoft.Web.WebView2.Core.dll'
    Add-Type -Path $core
    [Microsoft.Web.WebView2.Core.CoreWebView2Environment]::SetLoaderDllFolderPath($SdkDirectory)
    Add-Type -ReferencedAssemblies @($core,'System.Windows.Forms','System.Drawing') @'
using System;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Net;
using System.Net.Sockets;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading.Tasks;
using System.Windows.Forms;
using Microsoft.Web.WebView2.Core;

public sealed class PageDockLocalOrigin : IDisposable {
    TcpListener listener = new TcpListener(IPAddress.Loopback,0);
    public string Url;
    public PageDockLocalOrigin() {
        listener.Start();
        Url = "http://127.0.0.1:" + ((IPEndPoint)listener.LocalEndpoint).Port;
        Serve();
    }
    async void Serve() {
        try {
            while (true) {
                var client = await listener.AcceptTcpClientAsync();
                System.Threading.ThreadPool.QueueUserWorkItem(_ => Respond(client));
            }
        } catch (ObjectDisposedException) { }
          catch (SocketException) { }
    }
    void Respond(TcpClient client) {
        try {
            using (client) {
                client.ReceiveTimeout = 3000;
                using (var stream = client.GetStream())
                using (var reader = new StreamReader(stream,Encoding.ASCII,false,1024,true)) {
                    string line;
                    do { line = reader.ReadLine(); } while (!string.IsNullOrEmpty(line));
                    const string body = "<!doctype html><p>Isolated local navigation</p>";
                    byte[] data = Encoding.ASCII.GetBytes("HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nCache-Control: no-store\r\nConnection: close\r\nContent-Length: " + body.Length + "\r\n\r\n" + body);
                    stream.Write(data,0,data.Length);
                }
            }
        } catch (IOException) { /* A browser may abandon a preconnect. */ }
    }
    public void Dispose() { listener.Stop(); }
}

public sealed class PageDockVisualTree : IDisposable {
    [DllImport("dcomp.dll")] static extern int DCompositionCreateDevice2(IntPtr rendering, ref Guid iid, out IntPtr device);
    [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int CreateTarget(IntPtr self, IntPtr hwnd, int topmost, out IntPtr target);
    [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int CreateVisual(IntPtr self, out IntPtr visual);
    [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int SetRoot(IntPtr self, IntPtr root);
    [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int Commit(IntPtr self);
    IntPtr device, target, root;
    const int CommitIndex = 3, CreateTargetIndex = 6, CreateVisualIndex = 7, SetRootIndex = 3;
    static T Method<T>(IntPtr value, int index) where T : class {
        return Marshal.GetDelegateForFunctionPointer(Marshal.ReadIntPtr(Marshal.ReadIntPtr(value),index*IntPtr.Size),typeof(T)) as T;
    }
    public PageDockVisualTree(IntPtr hwnd, CoreWebView2CompositionController controller) {
        try {
            var iid = new Guid("c37ea93a-e7aa-450d-b16f-9746cb0407f3");
            Marshal.ThrowExceptionForHR(DCompositionCreateDevice2(IntPtr.Zero,ref iid,out device));
            Marshal.ThrowExceptionForHR(Method<CreateTarget>(device,CreateTargetIndex)(device,hwnd,0,out target));
            Marshal.ThrowExceptionForHR(Method<CreateVisual>(device,CreateVisualIndex)(device,out root));
            Marshal.ThrowExceptionForHR(Method<SetRoot>(target,SetRootIndex)(target,root));
            object visual = Marshal.GetObjectForIUnknown(root);
            try { controller.RootVisualTarget = visual; }
            finally { Marshal.ReleaseComObject(visual); }
            Marshal.ThrowExceptionForHR(Method<Commit>(device,CommitIndex)(device));
        } catch { Dispose(); throw; }
    }
    public void Dispose() {
        foreach (IntPtr value in new[] { root,target,device }) if (value != IntPtr.Zero) Marshal.Release(value);
        root = target = device = IntPtr.Zero;
    }
    public void CommitChanges() { Marshal.ThrowExceptionForHR(Method<Commit>(device,CommitIndex)(device)); }
}

public class PageDockProbeWindow : Form {
    protected override bool ShowWithoutActivation { get { return true; } }
}

public sealed class PageDockRuntimeProbe : PageDockProbeWindow {
    string output;
    int cycles, recovery;
    bool composition, visualTree, navigateOrigins, continuous;
    PageDockVisualTree tree;
    PageDockLocalOrigin source, foreign;
    StreamWriter log;
    Stopwatch watch = Stopwatch.StartNew();
    CoreWebView2Environment environment;
    CoreWebView2Controller controller;
    PageDockProbeWindow host;
    public bool RecreateWindow, ReadPermissions, EvaluateScript, RequestFilter, DetachVisualRoot;
    public string TargetVersion;
    public int Result = 2;
    protected override bool ShowWithoutActivation { get { return true; } }

    public PageDockRuntimeProbe(string path, int count, int idle, bool visual, bool attach, bool navigate, bool overlap) {
        output = path; cycles = count; recovery = idle;
        composition = visual; visualTree = attach; navigateOrigins = navigate; continuous = overlap;
        Text = "PageDock isolated WebView2 runtime probe";
        ShowInTaskbar = false; Opacity = 0;
        log = new StreamWriter(Path.Combine(output,"native-memory.jsonl"));
        log.AutoFlush = true;
        Shown += async delegate {
            try { await Work(); Result = 0; }
            catch (Exception error) { File.WriteAllText(Path.Combine(output,"error.txt"),error.ToString()); }
            finally {
                if (controller != null) controller.Close();
                if (tree != null) tree.Dispose();
                if (host != null) host.Dispose();
                if (source != null) source.Dispose();
                if (foreign != null) foreign.Dispose();
                log.Dispose(); Close();
            }
        };
    }

    void Sample(string phase, int cycle) {
        using (Process process = Process.GetCurrentProcess()) {
            log.WriteLine(string.Format(CultureInfo.InvariantCulture,
                "{{\"phase\":\"{0}\",\"cycle\":{1},\"elapsed_s\":{2:F3},\"handles\":{3},\"private_bytes\":{4}}}",
                phase,cycle,watch.Elapsed.TotalSeconds,process.HandleCount,process.PrivateMemorySize64));
        }
    }

    async Task Wait(Task task) {
        if (await Task.WhenAny(task, Task.Delay(20000)) != task) throw new TimeoutException("WebView2 operation");
        await task;
    }

    async Task<CoreWebView2Controller> CreateController() {
        IntPtr parent = host == null ? Handle : host.Handle;
        if (composition) return await environment.CreateCoreWebView2CompositionControllerAsync(parent);
        return await environment.CreateCoreWebView2ControllerAsync(parent);
    }

    public double CyclePace = 0.1, Dwell;

    async Task Work() {
        File.WriteAllText(Path.Combine(output,"pid"),Process.GetCurrentProcess().Id.ToString());
        Sample("cold",0);
        var options = new CoreWebView2EnvironmentOptions();
        if (!string.IsNullOrEmpty(TargetVersion)) options.TargetCompatibleBrowserVersion = TargetVersion;
        environment = await CoreWebView2Environment.CreateAsync(null, Path.Combine(output,"profile"), options);
        File.WriteAllText(Path.Combine(output,"runtime-version.txt"),environment.BrowserVersionString);
        if (navigateOrigins) { source = new PageDockLocalOrigin(); foreign = new PageDockLocalOrigin(); }
        for (int cycle = 1; cycle <= cycles; cycle++) {
            var cycleTime = Stopwatch.StartNew();
            if (RecreateWindow) {
                host = new PageDockProbeWindow();
                host.Text = "PageDock isolated native host";
                host.Size = new System.Drawing.Size(640,480);
                host.Show();
            }
            var creation = CreateController();
            await Wait(creation);
            controller = await creation;
            if (RequestFilter) {
                controller.CoreWebView2.WebResourceRequested += (sender,args) => { };
                controller.CoreWebView2.AddWebResourceRequestedFilter("http://mygo.localhost/*",CoreWebView2WebResourceContext.All);
            }
            if (ReadPermissions) await Wait(controller.CoreWebView2.Profile.GetNonDefaultPermissionSettingsAsync());
            if (visualTree) tree = new PageDockVisualTree(host == null ? Handle : host.Handle,(CoreWebView2CompositionController)controller);
            controller.Bounds = new System.Drawing.Rectangle(0,0,640,480);
            controller.IsVisible = visualTree;
            for (int visit = 0; visit < (navigateOrigins ? 4 : 1); visit++) {
                var complete = new TaskCompletionSource<bool>();
                EventHandler<CoreWebView2NavigationCompletedEventArgs> onNavigation = (sender,args) => {
                    if (args.IsSuccess) complete.TrySetResult(true);
                    else complete.TrySetException(new Exception("Navigation " + args.WebErrorStatus));
                };
                controller.CoreWebView2.NavigationCompleted += onNavigation;
                if (navigateOrigins) controller.CoreWebView2.Navigate((visit == 3 ? foreign.Url : source.Url) + "/" + cycle + "/" + visit);
                else controller.CoreWebView2.NavigateToString("<!doctype html><title>Isolated local runtime probe</title><p>cycle " + cycle + "</p>");
                await Wait(complete.Task);
                controller.CoreWebView2.NavigationCompleted -= onNavigation;
                if (EvaluateScript) await Wait(controller.CoreWebView2.CallDevToolsProtocolMethodAsync(
                    "Runtime.evaluate", "{\"expression\":\"(async () => document.readyState)()\",\"awaitPromise\":true,\"returnByValue\":true}"));
            }
            if (Dwell > 0) await Task.Delay((int)(Dwell * 1000));
            uint browser = controller.CoreWebView2.BrowserProcessId;
            if (host != null) { host.Hide(); host.Show(); }
            if (DetachVisualRoot && composition) {
                ((CoreWebView2CompositionController)controller).RootVisualTarget = null;
                if (tree != null) tree.CommitChanges();
            }
            controller.Close();
            controller = null;
            if (tree != null) { tree.Dispose(); tree = null; }
            if (host != null) { host.Dispose(); host = null; }
            GC.Collect(); GC.WaitForPendingFinalizers(); GC.Collect();
            if (!continuous) try {
                using (Process process = Process.GetProcessById((int)browser)) {
                    var deadline = Stopwatch.StartNew();
                    while (!process.HasExited && deadline.Elapsed.TotalSeconds < 20) await Task.Delay(50);
                    if (!process.HasExited) throw new TimeoutException("Browser process exit");
                }
            } catch (ArgumentException) { /* Already exited. */ }
            await Task.Delay(Math.Max(100,(int)(CyclePace * 1000 - cycleTime.ElapsedMilliseconds)));
            Sample("closed",cycle);
        }
        GC.Collect(); GC.WaitForPendingFinalizers(); GC.Collect();
        for (int second = 0; second < recovery; second++) {
            await Task.Delay(1000);
            if ((second + 1) % 10 == 0 || second + 1 == recovery) Sample("idle",cycles);
        }
        environment = null;
    }

    public static int Run(string path, int count, int idle, bool visual, bool attach, bool navigate, bool overlap, double pace, double dwell, bool newWindow, bool permissions, string targetVersion, bool evaluate, bool filter, bool detach) {
        using (var form = new PageDockRuntimeProbe(path,count,idle,visual,attach,navigate,overlap)) {
            form.CyclePace = pace;
            form.Dwell = dwell;
            form.RecreateWindow = newWindow;
            form.ReadPermissions = permissions;
            form.EvaluateScript = evaluate;
            form.RequestFilter = filter;
            form.DetachVisualRoot = detach;
            form.TargetVersion = targetVersion;
            Application.Run(form);
            return form.Result;
        }
    }
}
'@
    $code = [PageDockRuntimeProbe]::Run($Output,$Cycles,$Recovery,$Composition.IsPresent,$VisualTree.IsPresent,$NavigateOrigins.IsPresent,$Continuous.IsPresent,$CyclePace,$Dwell,$RecreateWindow.IsPresent,$ReadPermissions.IsPresent,$TargetVersion,$EvaluateScript.IsPresent,$RequestFilter.IsPresent,$DetachVisualRoot.IsPresent)
    @{exit_code=$code; cycles=$Cycles; recovery_s=$Recovery; os=[Environment]::OSVersion.VersionString;
      composition=$Composition.IsPresent; visual_tree=$VisualTree.IsPresent; loopback_navigation=$NavigateOrigins.IsPresent; continuous=$Continuous.IsPresent; cycle_pace=$CyclePace;
      recreate_window=$RecreateWindow.IsPresent; dwell_s=$Dwell;
      read_permissions=$ReadPermissions.IsPresent; evaluate_script=$EvaluateScript.IsPresent;
      request_filter=$RequestFilter.IsPresent; target_version=$TargetVersion; detach_visual_root=$DetachVisualRoot.IsPresent;
      sdk=$core; process_architecture=$env:PROCESSOR_ARCHITECTURE; boundary='official SDK, one shared environment, local HTML; no Go/MyGo/PageDock'} |
        ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $Output 'result.json')
    if ($code -ne 0) { throw "Native runtime probe failed; inspect $Output\error.txt" }
} finally {
    foreach ($key in $saved.Keys) { [Environment]::SetEnvironmentVariable($key,$saved[$key],'Process') }
}
