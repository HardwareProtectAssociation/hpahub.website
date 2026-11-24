$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
foreach ($command in @('go', 'node', 'npm.cmd')) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) {
        throw "Missing $command. Install Go 1.24+ and Node.js 22, then try again."
    }
}
if (-not (Test-Path (Join-Path $root 'frontend/node_modules/.bin/vite.cmd'))) {
    throw 'Frontend dependencies are missing. Run npm ci in the frontend directory first.'
}

$occupied = @(Get-NetTCPConnection -State Listen -LocalPort 5173, 8090 -ErrorAction SilentlyContinue)
if ($occupied.Count -gt 0) {
    $details = ($occupied | ForEach-Object { "port $($_.LocalPort), PID $($_.OwningProcess)" }) -join '; '
    throw "Development port already in use ($details). Inspect the process before stopping it."
}

# A job closes all descendants even when the console window is closed.
if (-not ('HpaDevJob' -as [type])) {
Add-Type -TypeDefinition @'
using System;
using System.ComponentModel;
using System.Runtime.InteropServices;
public static class HpaDevJob {
    [StructLayout(LayoutKind.Sequential)] struct Basic {
        public long processTime, jobTime; public uint flags; public UIntPtr min, max;
        public uint count; public UIntPtr affinity; public uint priority, scheduling;
    }
    [StructLayout(LayoutKind.Sequential)] struct IO { public ulong a,b,c,d,e,f; }
    [StructLayout(LayoutKind.Sequential)] struct Limits {
        public Basic basic; public IO io; public UIntPtr processMemory, jobMemory, peakProcess, peakJob;
    }
    [StructLayout(LayoutKind.Sequential, CharSet=CharSet.Unicode)] struct Startup {
        public int size; public string reserved, desktop, title;
        public int x,y,xsize,ysize,xchars,ychars,fill,flags; public short show,reserved2;
        public IntPtr reservedPtr,input,output,error;
    }
    [StructLayout(LayoutKind.Sequential)] struct Process { public IntPtr process, thread; public int id, threadId; }
    [DllImport("kernel32.dll", SetLastError=true, CharSet=CharSet.Unicode)] static extern IntPtr CreateJobObject(IntPtr security, string name);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool SetInformationJobObject(IntPtr job, int type, ref Limits limits, uint length);
    [DllImport("kernel32.dll", SetLastError=true, CharSet=CharSet.Unicode)] static extern bool CreateProcess(string app, string command, IntPtr pa, IntPtr ta, bool inherit, uint flags, IntPtr env, string dir, ref Startup startup, out Process process);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool AssignProcessToJobObject(IntPtr job, IntPtr process);
    [DllImport("kernel32.dll", SetLastError=true)] static extern uint ResumeThread(IntPtr thread);
    [DllImport("kernel32.dll")] static extern bool TerminateProcess(IntPtr process, uint code);
    [DllImport("kernel32.dll")] public static extern bool CloseHandle(IntPtr handle);
    public static IntPtr Create() {
        var job = CreateJobObject(IntPtr.Zero, null);
        if (job == IntPtr.Zero) throw new Win32Exception();
        var limits = new Limits(); limits.basic.flags = 0x2000;
        if (!SetInformationJobObject(job, 9, ref limits, (uint)Marshal.SizeOf(typeof(Limits)))) {
            int error = Marshal.GetLastWin32Error(); CloseHandle(job); throw new Win32Exception(error);
        }
        return job;
    }
    public static int Start(IntPtr job, string shell, string script, string dir) {
        var startup = new Startup(); startup.size = Marshal.SizeOf(typeof(Startup)); Process p;
        string command = "\"" + shell + "\" -NoLogo -NoProfile -ExecutionPolicy Bypass -File \"" + script + "\"";
        if (!CreateProcess(shell, command, IntPtr.Zero, IntPtr.Zero, false, 4, IntPtr.Zero, dir, ref startup, out p)) throw new Win32Exception();
        try {
            if (!AssignProcessToJobObject(job, p.process)) throw new Win32Exception();
            if (ResumeThread(p.thread) == UInt32.MaxValue) throw new Win32Exception();
            return p.id;
        } catch { TerminateProcess(p.process, 1); throw; }
        finally { CloseHandle(p.thread); CloseHandle(p.process); }
    }
}
'@
}

$job = [HpaDevJob]::Create()
try {
    $shell = (Get-Process -Id $PID).Path
    $backend = [HpaDevJob]::Start($job, $shell, (Join-Path $root 'scripts/dev-backend.ps1'), $root)
    $frontend = [HpaDevJob]::Start($job, $shell, (Join-Path $root 'scripts/dev-frontend.ps1'), $root)
    Write-Host 'Starting development servers. Open http://127.0.0.1:5173'
    Write-Host 'Press Ctrl+C or close this window to stop both servers.'
    while ((Get-Process -Id $backend -ErrorAction SilentlyContinue) -and (Get-Process -Id $frontend -ErrorAction SilentlyContinue)) {
        Start-Sleep -Seconds 1
    }
    throw 'A development server exited. Stopping both servers; check the output above.'
} finally {
    [void][HpaDevJob]::CloseHandle($job)
}
