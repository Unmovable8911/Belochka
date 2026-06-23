package monitor

import (
	"testing"
)

// Sample /proc/stat output from Ubuntu 22.04 (16-core)
const procStatUbuntu = `cpu  246430 280 117421 10728825 3478 0 26750 0 0 0
cpu0 17562 6 9230 664462 237 0 22326 0 0 0
cpu1 13242 6 5957 675176 207 0 1906 0 0 0
cpu2 17521 17 9101 665616 254 0 1038 0 0 0
cpu3 13389 6 5582 675924 99 0 234 0 0 0
cpu4 17828 13 8877 665343 224 0 390 0 0 0
cpu5 14240 11 5767 675035 122 0 72 0 0 0
cpu6 16802 38 8846 666702 251 0 112 0 0 0
cpu7 12049 22 5524 677174 115 0 96 0 0 0
cpu8 17782 38 8785 665734 266 0 71 0 0 0
cpu9 13197 13 5543 676371 110 0 51 0 0 0
cpu10 17041 18 8738 666504 268 0 76 0 0 0
cpu11 12685 25 5614 675960 128 0 52 0 0 0
cpu12 16443 20 8625 667169 260 0 96 0 0 0
cpu13 12262 16 5519 676584 133 0 85 0 0 0
cpu14 15877 17 8459 667915 262 0 77 0 0 0
cpu15 12510 14 5253 676156 142 0 68 0 0 0
intr 41756498 0 9 0
ctxt 103726845
btime 1750795158
processes 37866
procs_running 2
procs_blocked 0
softirq 23840282 1073753 1758263 12 6505 3098628 0 134629 8755218 0 9013274
`

// Sample /proc/stat from CentOS 7 (2-core)
const procStatCentOS = `cpu  4032 50 1820 891234 112 0 340 0 0 0
cpu0 2100 30 980 445000 60 0 200 0 0 0
cpu1 1932 20 840 446234 52 0 140 0 0 0
intr 5000000 0 5 0
ctxt 8000000
btime 1700000000
processes 5000
procs_running 1
procs_blocked 0
softirq 3000000 100000 200000 0 500 300000 0 10000 1000000 0 1389500
`

func TestParseCPU(t *testing.T) {
	t.Run("ubuntu 16-core", func(t *testing.T) {
		cpu, err := ParseCPU(procStatUbuntu)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check aggregate
		if cpu.Aggregate.Name != "cpu" {
			t.Errorf("aggregate name = %q, want %q", cpu.Aggregate.Name, "cpu")
		}
		if cpu.Aggregate.User != 246430 {
			t.Errorf("aggregate user = %d, want %d", cpu.Aggregate.User, 246430)
		}
		if cpu.Aggregate.Nice != 280 {
			t.Errorf("aggregate nice = %d, want %d", cpu.Aggregate.Nice, 280)
		}
		if cpu.Aggregate.System != 117421 {
			t.Errorf("aggregate system = %d, want %d", cpu.Aggregate.System, 117421)
		}
		if cpu.Aggregate.Idle != 10728825 {
			t.Errorf("aggregate idle = %d, want %d", cpu.Aggregate.Idle, 10728825)
		}
		if cpu.Aggregate.IOWait != 3478 {
			t.Errorf("aggregate iowait = %d, want %d", cpu.Aggregate.IOWait, 3478)
		}
		if cpu.Aggregate.IRQ != 0 {
			t.Errorf("aggregate irq = %d, want %d", cpu.Aggregate.IRQ, 0)
		}
		if cpu.Aggregate.SoftIRQ != 26750 {
			t.Errorf("aggregate softirq = %d, want %d", cpu.Aggregate.SoftIRQ, 26750)
		}
		if cpu.Aggregate.Steal != 0 {
			t.Errorf("aggregate steal = %d, want %d", cpu.Aggregate.Steal, 0)
		}

		// Check per-core count
		if len(cpu.Cores) != 16 {
			t.Fatalf("core count = %d, want 16", len(cpu.Cores))
		}

		// Spot-check cpu0
		c0 := cpu.Cores[0]
		if c0.Name != "cpu0" {
			t.Errorf("core 0 name = %q, want %q", c0.Name, "cpu0")
		}
		if c0.User != 17562 {
			t.Errorf("core 0 user = %d, want %d", c0.User, 17562)
		}
		if c0.SoftIRQ != 22326 {
			t.Errorf("core 0 softirq = %d, want %d", c0.SoftIRQ, 22326)
		}

		// Spot-check cpu15 (last core)
		c15 := cpu.Cores[15]
		if c15.Name != "cpu15" {
			t.Errorf("core 15 name = %q, want %q", c15.Name, "cpu15")
		}
		if c15.User != 12510 {
			t.Errorf("core 15 user = %d, want %d", c15.User, 12510)
		}
	})

	t.Run("centos 2-core", func(t *testing.T) {
		cpu, err := ParseCPU(procStatCentOS)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(cpu.Cores) != 2 {
			t.Fatalf("core count = %d, want 2", len(cpu.Cores))
		}
		if cpu.Aggregate.User != 4032 {
			t.Errorf("aggregate user = %d, want %d", cpu.Aggregate.User, 4032)
		}
		if cpu.Cores[1].User != 1932 {
			t.Errorf("core 1 user = %d, want %d", cpu.Cores[1].User, 1932)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		_, err := ParseCPU("")
		if err == nil {
			t.Error("expected error for empty input")
		}
	})
}

// Sample /proc/meminfo from Ubuntu 22.04
const procMeminfoUbuntu = `MemTotal:       32689396 kB
MemFree:        17234568 kB
MemAvailable:   25678432 kB
Buffers:         1234567 kB
Cached:          6789012 kB
SwapCached:            0 kB
Active:          8765432 kB
Inactive:        4321098 kB
Active(anon):    5432109 kB
Inactive(anon):        0 kB
Active(file):    3333323 kB
Inactive(file):  4321098 kB
Unevictable:           0 kB
Mlocked:               0 kB
SwapTotal:       8388604 kB
SwapFree:        8000000 kB
Dirty:               100 kB
Writeback:             0 kB
AnonPages:       5432109 kB
Mapped:          1234567 kB
Shmem:            123456 kB
KReclaimable:     567890 kB
Slab:             890123 kB
SReclaimable:     567890 kB
SUnreclaim:       322233 kB
KernelStack:       12345 kB
PageTables:        34567 kB
NFS_Unstable:          0 kB
Bounce:                0 kB
WritebackTmp:          0 kB
CommitLimit:    24733300 kB
Committed_AS:   12345678 kB
VmallocTotal:   34359738367 kB
VmallocUsed:       56789 kB
VmallocChunk:          0 kB
HardwareCorrupted:     0 kB
AnonHugePages:         0 kB
ShmemHugePages:        0 kB
ShmemPmdMapped:        0 kB
CmaTotal:              0 kB
CmaFree:               0 kB
HugePages_Total:       0
HugePages_Free:        0
HugePages_Rsvd:        0
HugePages_Surp:        0
Hugepagesize:       2048 kB
DirectMap4k:      234567 kB
DirectMap2M:    12345678 kB
DirectMap1G:    20971520 kB
`

// Sample /proc/meminfo from CentOS 7 (no swap)
const procMeminfoCentOS = `MemTotal:        4038592 kB
MemFree:          512000 kB
MemAvailable:    2048000 kB
Buffers:          123456 kB
Cached:          1234567 kB
SwapCached:            0 kB
Active:          2000000 kB
Inactive:        1000000 kB
SwapTotal:             0 kB
SwapFree:              0 kB
`

func TestParseMemory(t *testing.T) {
	t.Run("ubuntu with swap", func(t *testing.T) {
		mem, err := ParseMemory(procMeminfoUbuntu)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Values are in kB in the file, parser should return bytes
		if mem.Total != 32689396*1024 {
			t.Errorf("total = %d, want %d", mem.Total, 32689396*1024)
		}
		if mem.Available != 25678432*1024 {
			t.Errorf("available = %d, want %d", mem.Available, 25678432*1024)
		}
		// Used = Total - Available
		wantUsed := uint64((32689396 - 25678432) * 1024)
		if mem.Used != wantUsed {
			t.Errorf("used = %d, want %d", mem.Used, wantUsed)
		}
		if mem.SwapTotal != 8388604*1024 {
			t.Errorf("swap total = %d, want %d", mem.SwapTotal, 8388604*1024)
		}
		// SwapUsed = SwapTotal - SwapFree
		wantSwapUsed := uint64((8388604 - 8000000) * 1024)
		if mem.SwapUsed != wantSwapUsed {
			t.Errorf("swap used = %d, want %d", mem.SwapUsed, wantSwapUsed)
		}
	})

	t.Run("centos no swap", func(t *testing.T) {
		mem, err := ParseMemory(procMeminfoCentOS)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if mem.Total != 4038592*1024 {
			t.Errorf("total = %d, want %d", mem.Total, 4038592*1024)
		}
		if mem.SwapTotal != 0 {
			t.Errorf("swap total = %d, want 0", mem.SwapTotal)
		}
		if mem.SwapUsed != 0 {
			t.Errorf("swap used = %d, want 0", mem.SwapUsed)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		_, err := ParseMemory("")
		if err == nil {
			t.Error("expected error for empty input")
		}
	})
}

// Sample df -B1 output from Ubuntu 22.04
const dfOutputUbuntu = `Filesystem     1B-blocks         Used    Available Use% Mounted on
/dev/sda1      214748364800  107374182400  96636764160  53% /
/dev/sdb1      536870912000  268435456000 241591910400  53% /data
/dev/sda2        1073741824    209715200    864026624  20% /boot
`

// Sample df -B1 output from Debian (single disk)
const dfOutputDebian = `Filesystem     1B-blocks      Used Available Use% Mounted on
/dev/vda1      21474836480 5368709120 15032385536  27% /
`

func TestParseDisk(t *testing.T) {
	t.Run("ubuntu multi-partition", func(t *testing.T) {
		disk, err := ParseDisk(dfOutputUbuntu)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(disk.Partitions) != 3 {
			t.Fatalf("partition count = %d, want 3", len(disk.Partitions))
		}

		p0 := disk.Partitions[0]
		if p0.Filesystem != "/dev/sda1" {
			t.Errorf("p0 filesystem = %q, want %q", p0.Filesystem, "/dev/sda1")
		}
		if p0.MountPoint != "/" {
			t.Errorf("p0 mount = %q, want %q", p0.MountPoint, "/")
		}
		if p0.Total != 214748364800 {
			t.Errorf("p0 total = %d, want %d", p0.Total, 214748364800)
		}
		if p0.Used != 107374182400 {
			t.Errorf("p0 used = %d, want %d", p0.Used, 107374182400)
		}
		if p0.Available != 96636764160 {
			t.Errorf("p0 available = %d, want %d", p0.Available, 96636764160)
		}

		p2 := disk.Partitions[2]
		if p2.MountPoint != "/boot" {
			t.Errorf("p2 mount = %q, want %q", p2.MountPoint, "/boot")
		}
	})

	t.Run("debian single disk", func(t *testing.T) {
		disk, err := ParseDisk(dfOutputDebian)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(disk.Partitions) != 1 {
			t.Fatalf("partition count = %d, want 1", len(disk.Partitions))
		}

		p := disk.Partitions[0]
		if p.MountPoint != "/" {
			t.Errorf("mount = %q, want %q", p.MountPoint, "/")
		}
		if p.Total != 21474836480 {
			t.Errorf("total = %d, want %d", p.Total, 21474836480)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		disk, err := ParseDisk("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(disk.Partitions) != 0 {
			t.Errorf("partition count = %d, want 0", len(disk.Partitions))
		}
	})
}

// Sample /proc/net/dev from Ubuntu 22.04
const procNetDevUbuntu = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1234567890  12345678    0    0    0     0          0         0 1234567890  12345678    0    0    0     0       0          0
  eth0: 98765432100  87654321    0    0    0     0          0         0 54321098765  43210987    0    0    0     0       0          0
  eth1: 11111111111  22222222    0    5    0     0          0         0 33333333333  44444444    0    0    0     0       0          0
docker0:   55555555    66666    0    0    0     0          0         0    77777777    88888    0    0    0     0       0          0
`

// Sample /proc/net/dev from CentOS (single interface)
const procNetDevCentOS = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:  500000   5000    0    0    0     0          0         0   500000   5000    0    0    0     0       0          0
  ens3: 4000000000  3000000    0    0    0     0          0         0 2000000000  1500000    0    0    0     0       0          0
`

func TestParseNetwork(t *testing.T) {
	t.Run("ubuntu multi-interface", func(t *testing.T) {
		net, err := ParseNetwork(procNetDevUbuntu)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(net.Interfaces) != 4 {
			t.Fatalf("interface count = %d, want 4", len(net.Interfaces))
		}

		// lo
		lo := net.Interfaces[0]
		if lo.Name != "lo" {
			t.Errorf("iface 0 name = %q, want %q", lo.Name, "lo")
		}
		if lo.RxBytes != 1234567890 {
			t.Errorf("lo rx = %d, want %d", lo.RxBytes, 1234567890)
		}
		if lo.TxBytes != 1234567890 {
			t.Errorf("lo tx = %d, want %d", lo.TxBytes, 1234567890)
		}

		// eth0
		eth0 := net.Interfaces[1]
		if eth0.Name != "eth0" {
			t.Errorf("iface 1 name = %q, want %q", eth0.Name, "eth0")
		}
		if eth0.RxBytes != 98765432100 {
			t.Errorf("eth0 rx = %d, want %d", eth0.RxBytes, 98765432100)
		}
		if eth0.TxBytes != 54321098765 {
			t.Errorf("eth0 tx = %d, want %d", eth0.TxBytes, 54321098765)
		}

		// docker0
		docker := net.Interfaces[3]
		if docker.Name != "docker0" {
			t.Errorf("iface 3 name = %q, want %q", docker.Name, "docker0")
		}
	})

	t.Run("centos single interface", func(t *testing.T) {
		net, err := ParseNetwork(procNetDevCentOS)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(net.Interfaces) != 2 {
			t.Fatalf("interface count = %d, want 2", len(net.Interfaces))
		}

		ens3 := net.Interfaces[1]
		if ens3.Name != "ens3" {
			t.Errorf("iface 1 name = %q, want %q", ens3.Name, "ens3")
		}
		if ens3.RxBytes != 4000000000 {
			t.Errorf("ens3 rx = %d, want %d", ens3.RxBytes, 4000000000)
		}
		if ens3.TxBytes != 2000000000 {
			t.Errorf("ens3 tx = %d, want %d", ens3.TxBytes, 2000000000)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		net, err := ParseNetwork("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(net.Interfaces) != 0 {
			t.Errorf("interface count = %d, want 0", len(net.Interfaces))
		}
	})
}

// Sample top -bn1 -o %CPU | head -27 output from Ubuntu (procps-ng)
const topOutputUbuntu = `top - 14:32:01 up 2 days,  3:45,  2 users,  load average: 1.23, 0.98, 0.76
Tasks: 234 total,   2 running, 230 sleeping,   0 stopped,   2 zombie
%Cpu(s):  5.3 us,  2.1 sy,  0.0 ni, 91.8 id,  0.5 wa,  0.0 hi,  0.3 si,  0.0 st
MiB Mem :  31923.2 total,  16830.6 free,   8234.5 used,   6858.1 buff/cache
MiB Swap:   8192.0 total,   8192.0 free,      0.0 used.  22456.3 avail Mem

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
   1234 root      20   0 4567890 234567  12345 S  25.3   0.7   1:23.45 java
   5678 www-data  20   0  987654 123456  67890 S  12.1   0.4   0:45.67 nginx
   9012 postgres  20   0  345678  89012  45678 S   8.5   0.3   2:34.56 postgres
   3456 root      20   0  234567  67890  34567 R   5.2   0.2   0:12.34 python3
   7890 kilian    20   0  123456  45678  23456 S   3.1   0.1   0:05.67 vim
   2345 root      20   0   98765  34567  12345 S   1.5   0.1   0:02.34 systemd
   6789 nobody    20   0   87654  23456   9876 S   0.8   0.1   0:01.23 dnsmasq
`

// Sample top output from Debian (procps-ng, slightly different header)
const topOutputDebian = `top - 10:15:33 up 45 days,  7:22,  1 user,  load average: 0.15, 0.10, 0.05
Tasks:  95 total,   1 running,  94 sleeping,   0 stopped,   0 zombie
%Cpu(s):  1.2 us,  0.8 sy,  0.0 ni, 97.8 id,  0.1 wa,  0.0 hi,  0.1 si,  0.0 st
MiB Mem :   3940.0 total,    500.0 free,   1960.0 used,   1480.0 buff/cache
MiB Swap:      0.0 total,      0.0 free,      0.0 used.   1700.0 avail Mem

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
    100 root      20   0  100000  50000  25000 S   2.0   1.3   0:30.00 node
    200 www-data  20   0   80000  40000  20000 S   1.0   1.0   0:15.00 apache2
`

// BusyBox top output (completely different format)
const topOutputBusyBox = `Mem: 1003296K used, 20288K free, 1208K shrd, 49944K buff, 608488K cached
CPU:   2% usr   1% sys   0% nic  96% idle   0% io   0% irq   0% sirq
Load average: 0.08 0.03 0.01 2/127 31352
  PID  PPID USER     STAT   VSZ %VSZ %CPU COMMAND
31349 31340 root     R     1576   0%   0% top -bn1
    1     0 root     S     1600   0%   0% /sbin/init
`

func TestParseProcesses(t *testing.T) {
	t.Run("ubuntu procps-ng", func(t *testing.T) {
		procs, err := ParseProcesses(topOutputUbuntu)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(procs.Processes) != 7 {
			t.Fatalf("process count = %d, want 7", len(procs.Processes))
		}

		p0 := procs.Processes[0]
		if p0.PID != 1234 {
			t.Errorf("p0 pid = %d, want %d", p0.PID, 1234)
		}
		if p0.User != "root" {
			t.Errorf("p0 user = %q, want %q", p0.User, "root")
		}
		if p0.CPUPct != 25.3 {
			t.Errorf("p0 cpu = %f, want %f", p0.CPUPct, 25.3)
		}
		if p0.MemPct != 0.7 {
			t.Errorf("p0 mem = %f, want %f", p0.MemPct, 0.7)
		}
		if p0.Command != "java" {
			t.Errorf("p0 command = %q, want %q", p0.Command, "java")
		}

		// Check last process
		pLast := procs.Processes[6]
		if pLast.PID != 6789 {
			t.Errorf("last pid = %d, want %d", pLast.PID, 6789)
		}
		if pLast.Command != "dnsmasq" {
			t.Errorf("last command = %q, want %q", pLast.Command, "dnsmasq")
		}
	})

	t.Run("debian procps-ng", func(t *testing.T) {
		procs, err := ParseProcesses(topOutputDebian)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(procs.Processes) != 2 {
			t.Fatalf("process count = %d, want 2", len(procs.Processes))
		}

		if procs.Processes[0].PID != 100 {
			t.Errorf("p0 pid = %d, want %d", procs.Processes[0].PID, 100)
		}
	})

	t.Run("busybox graceful empty", func(t *testing.T) {
		procs, err := ParseProcesses(topOutputBusyBox)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(procs.Processes) != 0 {
			t.Errorf("process count = %d, want 0 for BusyBox output", len(procs.Processes))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		procs, err := ParseProcesses("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(procs.Processes) != 0 {
			t.Errorf("process count = %d, want 0", len(procs.Processes))
		}
	})
}

// /etc/os-release from Ubuntu 22.04
const osReleaseUbuntu = `PRETTY_NAME="Ubuntu 22.04.3 LTS"
NAME="Ubuntu"
VERSION_ID="22.04"
VERSION="22.04.3 LTS (Jammy Jellyfish)"
VERSION_CODENAME=jammy
ID=ubuntu
ID_LIKE=debian
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
UBUNTU_CODENAME=jammy
`

// /etc/os-release from CentOS 7
const osReleaseCentOS = `NAME="CentOS Linux"
VERSION="7 (Core)"
ID="centos"
ID_LIKE="rhel fedora"
VERSION_ID="7"
PRETTY_NAME="CentOS Linux 7 (Core)"
ANSI_COLOR="0;31"
CPE_NAME="cpe:/o:centos:centos:7"
HOME_URL="https://www.centos.org/"
BUG_REPORT_URL="https://bugs.centos.org/"
CENTOS_MANTISBT_PROJECT="CentOS-7"
CENTOS_MANTISBT_PROJECT_VERSION="7"
REDHAT_SUPPORT_PRODUCT="centos"
REDHAT_SUPPORT_PRODUCT_VERSION="7"
`

// /etc/os-release from Debian 12
const osReleaseDebian = `PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
NAME="Debian GNU/Linux"
VERSION_ID="12"
VERSION="12 (bookworm)"
VERSION_CODENAME=bookworm
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`

func TestParseSystemInfo(t *testing.T) {
	t.Run("ubuntu full", func(t *testing.T) {
		info, err := ParseSystemInfo(
			"web-server-01",       // hostname
			"5.15.0-91-generic",   // uname -r
			"178901.23 1423209.84", // /proc/uptime
			osReleaseUbuntu,       // /etc/os-release
			"16",                  // nproc (core count)
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if info.Hostname != "web-server-01" {
			t.Errorf("hostname = %q, want %q", info.Hostname, "web-server-01")
		}
		if info.Kernel != "5.15.0-91-generic" {
			t.Errorf("kernel = %q, want %q", info.Kernel, "5.15.0-91-generic")
		}
		if info.UptimeSec != 178901.23 {
			t.Errorf("uptime = %f, want %f", info.UptimeSec, 178901.23)
		}
		if info.OSName != "Ubuntu 22.04.3 LTS" {
			t.Errorf("os name = %q, want %q", info.OSName, "Ubuntu 22.04.3 LTS")
		}
		if info.CoreCount != 16 {
			t.Errorf("core count = %d, want %d", info.CoreCount, 16)
		}
	})

	t.Run("centos", func(t *testing.T) {
		info, err := ParseSystemInfo(
			"db-server",
			"3.10.0-1160.el7.x86_64",
			"3888000.50 7000000.00",
			osReleaseCentOS,
			"2",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if info.OSName != "CentOS Linux 7 (Core)" {
			t.Errorf("os name = %q, want %q", info.OSName, "CentOS Linux 7 (Core)")
		}
		if info.CoreCount != 2 {
			t.Errorf("core count = %d, want %d", info.CoreCount, 2)
		}
	})

	t.Run("debian", func(t *testing.T) {
		info, err := ParseSystemInfo(
			"app-01",
			"6.1.0-17-amd64",
			"100.00 200.00",
			osReleaseDebian,
			"4",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if info.OSName != "Debian GNU/Linux 12 (bookworm)" {
			t.Errorf("os name = %q, want %q", info.OSName, "Debian GNU/Linux 12 (bookworm)")
		}
		if info.UptimeSec != 100.00 {
			t.Errorf("uptime = %f, want %f", info.UptimeSec, 100.00)
		}
	})

	t.Run("missing os-release falls back gracefully", func(t *testing.T) {
		info, err := ParseSystemInfo(
			"minimal-host",
			"5.10.0",
			"50.0 100.0",
			"",
			"1",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if info.OSName != "" {
			t.Errorf("os name = %q, want empty string", info.OSName)
		}
		if info.Hostname != "minimal-host" {
			t.Errorf("hostname = %q, want %q", info.Hostname, "minimal-host")
		}
	})
}
