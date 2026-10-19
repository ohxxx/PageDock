# [INPUT]: One explicitly selected test-process ID.
# [OUTPUT]: Kernel handle types, optional process IDs/exit states, and optional diagnostic object names.
# [POS]: Read-only diagnostic for resource growth in the Windows endurance workload.
# [PROTOCOL]: Update tests/soak/AGENTS.md when native diagnostic boundaries change.
param([Parameter(Mandatory=$true)][int]$ProcessId, [switch]$ProcessTargets, [switch]$ObjectNames)
$ErrorActionPreference = 'Stop'
Add-Type @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
public static class PageDockHandleTypes {
    [DllImport("kernel32.dll", SetLastError=true)] static extern IntPtr OpenProcess(uint access, bool inherit, int pid);
    [DllImport("kernel32.dll")] static extern IntPtr GetCurrentProcess();
    [DllImport("kernel32.dll")] static extern uint GetProcessId(IntPtr process);
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode)] static extern bool QueryFullProcessImageName(IntPtr process, uint flags, StringBuilder path, ref uint size);
    [DllImport("kernel32.dll")] static extern uint WaitForSingleObject(IntPtr handle, uint timeout);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool DuplicateHandle(IntPtr source, IntPtr handle, IntPtr target, out IntPtr copy, uint access, bool inherit, uint options);
    [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr handle);
    [DllImport("ntdll.dll")] static extern int NtQueryInformationProcess(IntPtr process, int info, IntPtr buffer, uint size, out uint needed);
    [DllImport("ntdll.dll")] static extern int NtQueryObject(IntPtr handle, int info, IntPtr buffer, uint size, out uint needed);
    public static Dictionary<string,int> Read(int pid, bool processTargets, bool objectNames) {
        if (IntPtr.Size != 8) throw new InvalidOperationException("64-bit PowerShell required");
        IntPtr process = OpenProcess(0x440, false, pid);
        if (process == IntPtr.Zero) throw new System.ComponentModel.Win32Exception();
        IntPtr buffer = IntPtr.Zero;
        try {
            uint size = 65536, needed;
            int status;
            while (true) {
                buffer = Marshal.AllocHGlobal((int)size);
                status = NtQueryInformationProcess(process, 51, buffer, size, out needed);
                if (status == 0) break;
                Marshal.FreeHGlobal(buffer); buffer = IntPtr.Zero;
                if (status != unchecked((int)0xC0000004) || size >= 16777216) throw new InvalidOperationException("Handle snapshot status " + status.ToString("X"));
                size = Math.Max(size * 2, needed);
            }
            long count = Marshal.ReadInt64(buffer);
            var types = new Dictionary<int,string>();
            var counts = new Dictionary<string,int>();
            for (int i=0; i<count; i++) {
                IntPtr entry = IntPtr.Add(buffer, 16 + i*40);
                int type = Marshal.ReadInt32(entry, 28);
                string name;
                if (!types.TryGetValue(type, out name)) {
                    IntPtr copy;
                    if (!DuplicateHandle(process, Marshal.ReadIntPtr(entry), GetCurrentProcess(), out copy, 0, false, 2)) continue;
                    IntPtr info = Marshal.AllocHGlobal(4096);
                    try {
                        status = NtQueryObject(copy, 2, info, 4096, out needed);
                        if (status != 0) throw new InvalidOperationException("Object type status " + status.ToString("X"));
                        name = Marshal.PtrToStringUni(Marshal.ReadIntPtr(info, 8), (ushort)Marshal.ReadInt16(info) / 2);
                        types[type] = name;
                    } finally { Marshal.FreeHGlobal(info); CloseHandle(copy); }
                }
                if (processTargets && name == "Process") {
                    IntPtr copy;
                    if (DuplicateHandle(process, Marshal.ReadIntPtr(entry), GetCurrentProcess(), out copy, 0, false, 2)) {
                        try {
                            uint state = WaitForSingleObject(copy, 0);
                            name += " pid=" + GetProcessId(copy) + (state == 0 ? " exited" : state == 258 ? " running" : " unknown");
                            if (objectNames) {
                                var path = new StringBuilder(32768); uint length = (uint)path.Capacity;
                                if (QueryFullProcessImageName(copy, 0, path, ref length)) name += " image=" + path;
                            }
                        } finally { CloseHandle(copy); }
                    }
                }
                // Query only Event names: some other object classes can block a name query.
                if (objectNames && name == "Event") {
                    IntPtr copy;
                    if (DuplicateHandle(process, Marshal.ReadIntPtr(entry), GetCurrentProcess(), out copy, 0, false, 2)) {
                        IntPtr info = Marshal.AllocHGlobal(65536);
                        try {
                            if (NtQueryObject(copy, 1, info, 65536, out needed) == 0) {
                                int length = (ushort)Marshal.ReadInt16(info);
                                if (length > 0) name += " name=" + Marshal.PtrToStringUni(Marshal.ReadIntPtr(info, 8), length / 2);
                            }
                        } finally { Marshal.FreeHGlobal(info); CloseHandle(copy); }
                    }
                }
                int value;
                counts.TryGetValue(name, out value);
                counts[name] = value + 1;
            }
            return counts;
        } finally { if (buffer != IntPtr.Zero) Marshal.FreeHGlobal(buffer); CloseHandle(process); }
    }
}
'@
[PageDockHandleTypes]::Read($ProcessId, $ProcessTargets.IsPresent, $ObjectNames.IsPresent) | ConvertTo-Json
