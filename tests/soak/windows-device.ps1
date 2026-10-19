# [INPUT]: An unused output directory and an explicit hardware/WARP D3D11 selection.
# [OUTPUT]: Handle counts after standalone device/context creation and release, without MyGo or WebView2.
# [POS]: Native graphics ownership diagnostic; no windows, browser profiles, or driver settings are changed.
# [PROTOCOL]: Update tests/soak/AGENTS.md and record device-only results separately from application endurance.
param([Parameter(Mandatory=$true)][string]$Output, [switch]$Software, [ValidateRange(1,1000)][int]$Cycles=80)
$ErrorActionPreference='Stop'
$Output=[IO.Path]::GetFullPath($Output)
if(Test-Path -LiteralPath $Output){throw 'Evidence directory must be new'}
New-Item -ItemType Directory -Path $Output | Out-Null
Add-Type @'
using System;
using System.Diagnostics;
using System.IO;
using System.Runtime.InteropServices;
using System.Threading;
public static class PageDockDeviceProbe {
    [DllImport("d3d11.dll")] static extern int D3D11CreateDevice(IntPtr adapter, uint driver, IntPtr software,
        uint flags, uint[] levels, uint count, uint sdk, out IntPtr device, out uint level, out IntPtr context);
    [UnmanagedFunctionPointer(CallingConvention.StdCall)] delegate void ContextMethod(IntPtr context);
    static void Call(IntPtr value, int index) {
        var method=(ContextMethod)Marshal.GetDelegateForFunctionPointer(Marshal.ReadIntPtr(Marshal.ReadIntPtr(value),index*IntPtr.Size),typeof(ContextMethod));
        method(value);
    }
    public static void Run(string output, bool software, int cycles) {
        using(var log=new StreamWriter(Path.Combine(output,"handles.jsonl"))) {
            for(int i=0;i<cycles+10;i++) {
                IntPtr device=IntPtr.Zero, context=IntPtr.Zero; uint level;
                int deviceRefs=-1, contextRefs=-1;
                try {
                    Marshal.ThrowExceptionForHR(D3D11CreateDevice(IntPtr.Zero,software?5u:1u,IntPtr.Zero,0x20,
                        new uint[]{0xb000,0xa100,0xa000},3,7,out device,out level,out context));
                    Call(context,110); // ID3D11DeviceContext::ClearState.
                    Call(context,111); // ID3D11DeviceContext::Flush.
                } finally {
                    if(context!=IntPtr.Zero) contextRefs=Marshal.Release(context);
                    if(device!=IntPtr.Zero) deviceRefs=Marshal.Release(device);
                }
                Thread.Sleep(50);
                using(var p=Process.GetCurrentProcess()) {
                    log.WriteLine("{\"phase\":\"released\",\"cycle\":"+(i+1)+",\"handles\":"+p.HandleCount+
                        ",\"context_refs\":"+contextRefs+",\"device_refs\":"+deviceRefs+"}");
                }
            }
            GC.Collect(); GC.WaitForPendingFinalizers(); GC.Collect();
            Thread.Sleep(10000);
            using(var p=Process.GetCurrentProcess()) log.WriteLine("{\"phase\":\"idle\",\"handles\":"+p.HandleCount+"}");
        }
    }
}
'@
[PageDockDeviceProbe]::Run($Output,$Software.IsPresent,$Cycles)
@{result='completed';software=$Software.IsPresent;cycles=$Cycles;warmup=10;boundary='native D3D11 devices only'} | ConvertTo-Json | Set-Content -Encoding UTF8 "$Output\result.json"
