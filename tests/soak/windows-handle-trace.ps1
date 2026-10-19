# [INPUT]: Exact isolated test executable, PID, and explicit tracing action.
# [OUTPUT]: Process-handle event history and available stacks; current handle targets are marked separately.
# [POS]: Diagnostic-only native tracing; traced runs are not memory/performance baselines.
# [PROTOCOL]: Update tests/soak/AGENTS.md; layouts follow winsiderss/phnt ntpsapi.h.
param(
    [Parameter(Mandatory=$true)][int]$ProcessId,
    [Parameter(Mandatory=$true)][string]$Executable,
    [ValidateSet('Enable','Read','Disable')][string]$Action = 'Read'
)
$ErrorActionPreference = 'Stop'
$expected = (Resolve-Path -LiteralPath $Executable).Path
$process = Get-Process -Id $ProcessId -ErrorAction Stop
if ($process.MainModule.FileName -ne $expected) { throw 'PID does not match the isolated test executable' }
Add-Type @'
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Runtime.InteropServices;

public static class PageDockHandleTrace {
    const int HandleTracing = 32, HandleSnapshot = 51, TraceEntrySize = 160, TraceHeaderSize = 16;
    const uint ProcessAccess = 0x640, SameAccess = 2;
    [DllImport("kernel32.dll", SetLastError=true)] static extern IntPtr OpenProcess(uint access, bool inherit, int pid);
    [DllImport("kernel32.dll")] static extern IntPtr GetCurrentProcess();
    [DllImport("kernel32.dll")] static extern uint GetProcessId(IntPtr process);
    [DllImport("kernel32.dll")] static extern uint WaitForSingleObject(IntPtr handle, uint timeout);
    [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr handle);
    [DllImport("kernel32.dll")] static extern bool DuplicateHandle(IntPtr source, IntPtr handle, IntPtr target, out IntPtr copy, uint access, bool inherit, uint options);
    [DllImport("ntdll.dll")] static extern int NtSetInformationProcess(IntPtr process, int info, IntPtr buffer, uint size);
    [DllImport("ntdll.dll")] static extern int NtQueryInformationProcess(IntPtr process, int info, IntPtr buffer, uint size, out uint needed);

    static IntPtr Open(int pid) {
        if (IntPtr.Size != 8) throw new InvalidOperationException("64-bit PowerShell required");
        IntPtr process = OpenProcess(ProcessAccess,false,pid);
        if (process == IntPtr.Zero) throw new System.ComponentModel.Win32Exception();
        return process;
    }
    static void Check(int status) {
        if (status != 0) throw new InvalidOperationException("Native tracing status " + status.ToString("X"));
    }
    public static void Set(int pid, bool enabled) {
        IntPtr process = Open(pid), buffer = Marshal.AllocHGlobal(8);
        try {
            Marshal.WriteInt32(buffer,0,0);
            Marshal.WriteInt32(buffer,4,65536);
            Check(NtSetInformationProcess(process,HandleTracing,enabled ? buffer : IntPtr.Zero,enabled ? 8u : 0u));
        } finally { Marshal.FreeHGlobal(buffer); CloseHandle(process); }
    }

    public static object Read(int pid) {
        IntPtr process = Open(pid), snapshot = IntPtr.Zero;
        IntPtr query = Marshal.AllocHGlobal(65536);
        var result = new List<object>();
        var kinds = new Dictionary<string,int>();
        int processHandles = 0, traceEvents = 0, stackEvents = 0;
        try {
            uint size = 65536, needed;
            while (true) {
                snapshot = Marshal.AllocHGlobal((int)size);
                int status = NtQueryInformationProcess(process,HandleSnapshot,snapshot,size,out needed);
                if (status == 0) break;
                Marshal.FreeHGlobal(snapshot); snapshot = IntPtr.Zero;
                if (status != unchecked((int)0xC0000004) || size >= 16777216) Check(status);
                size = Math.Max(size*2,needed);
            }
            using (Process target = Process.GetProcessById(pid)) {
                ProcessModuleCollection modules = target.Modules;
                long count = Marshal.ReadInt64(snapshot);
                for (int i=0;i<count;i++) {
                    IntPtr handle = Marshal.ReadIntPtr(snapshot,16+i*40), copy;
                    if (!DuplicateHandle(process,handle,GetCurrentProcess(),out copy,0,false,SameAccess)) continue;
                    uint peer; bool exited;
                    try { peer = GetProcessId(copy); exited = peer != 0 && WaitForSingleObject(copy,0) == 0; }
                    finally { CloseHandle(copy); }
                    if (peer == 0) continue;
                    processHandles++;
                    Marshal.WriteIntPtr(query,handle);
                    int status = NtQueryInformationProcess(process,HandleTracing,query,65536,out needed);
                    if (status != 0) { result.Add(new {handle=handle.ToInt64().ToString("X"),current_target_pid=peer,error=status.ToString("X")}); continue; }
                    int traces = Marshal.ReadInt32(query,8);
                    if (traces < 0 || TraceHeaderSize + traces*TraceEntrySize > 65536) throw new InvalidOperationException("Invalid trace length");
                    traceEvents += traces;
                    for (int n=0;n<traces;n++) {
                        IntPtr entry = IntPtr.Add(query,TraceHeaderSize+n*TraceEntrySize);
                        int kind = Marshal.ReadInt32(entry,24);
                        string kindName = kind.ToString();
                        int total; kinds.TryGetValue(kindName,out total); kinds[kindName] = total+1;
                        long creator = Marshal.ReadInt64(entry,8), thread = Marshal.ReadInt64(entry,16);
                        var frames = new List<string>();
                        for (int frame=0;frame<16;frame++) {
                            long address = Marshal.ReadInt64(entry,32+frame*8);
                            if (address == 0) continue;
                            string label = address.ToString("X");
                            foreach (ProcessModule module in modules) {
                                long start = module.BaseAddress.ToInt64();
                                if (creator == pid && address >= start && address < start+module.ModuleMemorySize) {
                                    label = module.ModuleName + "+0x" + (address-start).ToString("X"); break;
                                }
                            }
                            frames.Add(label);
                        }
                        if (frames.Count > 0) stackEvents++;
                        // Handle values can be reused. Historical events do not prove
                        // ownership of the process target found in the current snapshot.
                        result.Add(new {handle=handle.ToInt64().ToString("X"),current_target_pid=peer,current_exited=exited,
                            event_type=kind,trace_index=n,creator_pid=creator,creator_thread=thread,frames=frames});
                    }
                }
            }
            return new {pid=pid,process_handles=processHandles,trace_events=traceEvents,stack_events=stackEvents,trace_kinds=kinds,records=result};
        } finally { if(snapshot != IntPtr.Zero) Marshal.FreeHGlobal(snapshot); Marshal.FreeHGlobal(query); CloseHandle(process); }
    }
}
'@
if ($Action -eq 'Read') {
    ConvertTo-Json -InputObject ([PageDockHandleTrace]::Read($ProcessId)) -Depth 6
} else {
    [PageDockHandleTrace]::Set($ProcessId, $Action -eq 'Enable')
    Write-Output ($Action + ' handle tracing for PID ' + $ProcessId)
}
