# [INPUT]: An unused output directory and hardware/WARP selection for isolated native windows.
# [OUTPUT]: Handle and COM reference counts after D3D11 flip swap chains draw, present, and close.
# [POS]: Standalone native renderer diagnostic without MyGo, Go, or WebView2.
# [PROTOCOL]: Update tests/soak/AGENTS.md; completion is not a resource-recovery assertion.
param([Parameter(Mandatory=$true)][string]$Output, [switch]$Software, [ValidateRange(1,1000)][int]$Cycles=80)
$ErrorActionPreference='Stop'
$Output=[IO.Path]::GetFullPath($Output)
if(Test-Path -LiteralPath $Output){throw 'Evidence directory must be new'}
New-Item -ItemType Directory -Path $Output | Out-Null
Add-Type -ReferencedAssemblies System.Windows.Forms,System.Drawing @'
using System;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.Runtime.InteropServices;
using System.Threading;
using System.Windows.Forms;
public static class PageDockSwapChainProbe {
    [DllImport("d3d11.dll")] static extern int D3D11CreateDevice(IntPtr adapter, uint driver, IntPtr software,
        uint flags, uint[] levels, uint count, uint sdk, out IntPtr device, out uint level, out IntPtr context);
    [StructLayout(LayoutKind.Sequential)] struct SwapDesc {
        public uint Width,Height,Format,Stereo,SampleCount,SampleQuality,Usage,BufferCount,Scaling,Effect,Alpha,Flags;
    }
    [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int GetAdapter(IntPtr self,out IntPtr adapter);
    [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int GetParent(IntPtr self,ref Guid iid,out IntPtr parent);
    [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int CreateSwap(IntPtr self,IntPtr device,IntPtr hwnd,ref SwapDesc desc,IntPtr full,IntPtr output,out IntPtr chain);
    [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int GetBuffer(IntPtr self,uint index,ref Guid iid,out IntPtr buffer);
    [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int CreateView(IntPtr self,IntPtr resource,IntPtr desc,out IntPtr view);
    [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate void ClearView(IntPtr self,IntPtr view,[In] float[] color);
    [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate int Present(IntPtr self,uint sync,uint flags);
    [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate void ContextMethod(IntPtr self);
    static T Method<T>(IntPtr value,int index) where T:class {
        return Marshal.GetDelegateForFunctionPointer(Marshal.ReadIntPtr(Marshal.ReadIntPtr(value),index*IntPtr.Size),typeof(T)) as T;
    }
    static int Free(ref IntPtr value) {
        if(value==IntPtr.Zero) return -1;
        int refs=Marshal.Release(value); value=IntPtr.Zero; return refs;
    }
    static void Pump(int milliseconds) {
        var deadline=DateTime.UtcNow.AddMilliseconds(milliseconds);
        while(DateTime.UtcNow<deadline) {Application.DoEvents();Thread.Sleep(5);}
    }
    public static void Run(string output,bool software,int cycles) {
        var dxgiID=new Guid("54ec77fa-1377-44e6-8c32-88fd5f44c84c");
        var factoryID=new Guid("50c83a1c-e072-4c48-87b0-3630fa36a6d0");
        var textureID=new Guid("6f15aaf2-d208-4e89-9ab4-489535d34f9c");
        using(var log=new StreamWriter(Path.Combine(output,"handles.jsonl"))) {
            log.AutoFlush=true;
            for(int i=0;i<cycles+10;i++) {
                IntPtr device=IntPtr.Zero,context=IntPtr.Zero,dxgi=IntPtr.Zero,adapter=IntPtr.Zero,
                    factory=IntPtr.Zero,chain=IntPtr.Zero,texture=IntPtr.Zero,view=IntPtr.Zero;
                int deviceRefs=-1,contextRefs=-1,chainRefs=-1; uint level;
                using(var window=new Form {ClientSize=new Size(400,300),Text="PageDock isolated swap-chain diagnostic",ShowInTaskbar=false}) {
                    try {
                        window.Show(); Pump(20);
                        Marshal.ThrowExceptionForHR(D3D11CreateDevice(IntPtr.Zero,software?5u:1u,IntPtr.Zero,0x20,
                            new uint[]{0xb000,0xa100,0xa000},3,7,out device,out level,out context));
                        Marshal.ThrowExceptionForHR(Marshal.QueryInterface(device,ref dxgiID,out dxgi));
                        Marshal.ThrowExceptionForHR(Method<GetAdapter>(dxgi,7)(dxgi,out adapter));
                        Marshal.ThrowExceptionForHR(Method<GetParent>(adapter,6)(adapter,ref factoryID,out factory));
                        var desc=new SwapDesc {Width=400,Height=300,Format=87,SampleCount=1,Usage=0x20,BufferCount=2,Effect=4};
                        Marshal.ThrowExceptionForHR(Method<CreateSwap>(factory,15)(factory,device,window.Handle,ref desc,IntPtr.Zero,IntPtr.Zero,out chain));
                        Marshal.ThrowExceptionForHR(Method<GetBuffer>(chain,9)(chain,0,ref textureID,out texture));
                        Marshal.ThrowExceptionForHR(Method<CreateView>(device,9)(device,texture,IntPtr.Zero,out view));
                        Method<ClearView>(context,50)(context,view,new float[]{0.2f,0.3f,0.4f,1});
                        Marshal.ThrowExceptionForHR(Method<Present>(chain,8)(chain,1,0));
                        Pump(50); window.Hide(); window.Show(); Pump(20);
                    } finally {
                        Free(ref view); Free(ref texture); chainRefs=Free(ref chain);
                        Free(ref factory); Free(ref adapter); Free(ref dxgi);
                        if(context!=IntPtr.Zero) {Method<ContextMethod>(context,110)(context);Method<ContextMethod>(context,111)(context);}
                        contextRefs=Free(ref context); deviceRefs=Free(ref device);
                    }
                }
                Pump(20);
                using(var p=Process.GetCurrentProcess()) log.WriteLine("{\"phase\":\"released\",\"cycle\":"+(i+1)+",\"handles\":"+p.HandleCount+
                    ",\"context_refs\":"+contextRefs+",\"device_refs\":"+deviceRefs+",\"chain_refs\":"+chainRefs+"}");
            }
            GC.Collect();GC.WaitForPendingFinalizers();GC.Collect();Pump(10000);
            using(var p=Process.GetCurrentProcess()) log.WriteLine("{\"phase\":\"idle\",\"handles\":"+p.HandleCount+"}");
        }
    }
}
'@
[PageDockSwapChainProbe]::Run($Output,$Software.IsPresent,$Cycles)
@{result='completed';software=$Software.IsPresent;cycles=$Cycles;warmup=10;boundary='native D3D11 flip swap chains only'} |
    ConvertTo-Json | Set-Content -Encoding UTF8 "$Output\result.json"
