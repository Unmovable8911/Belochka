package parser_test

import (
	"testing"

	"belochka/internal/monitor/parser"
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
		cpu, err := parser.ParseCPU(procStatUbuntu)
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
		cpu, err := parser.ParseCPU(procStatCentOS)
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
		_, err := parser.ParseCPU("")
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
		mem, err := parser.ParseMemory(procMeminfoUbuntu)
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
		mem, err := parser.ParseMemory(procMeminfoCentOS)
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
		_, err := parser.ParseMemory("")
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
		disk, err := parser.ParseDisk(dfOutputUbuntu)
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
		disk, err := parser.ParseDisk(dfOutputDebian)
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
		disk, err := parser.ParseDisk("")
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

// Sample /proc/net/dev from a Docker host with VPN, KVM, and hand-named bridge.
const procNetDevDocker = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
  eth0: 1000000    1000    0    0    0     0          0         0   500000    500    0    0    0     0       0          0
docker0:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
br-29a590b46462:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
veth83f5d1f:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
  tun0:  500000    500    0    0    0     0          0         0   300000    300    0    0    0     0       0          0
 tap0:  100000    100    0    0    0     0          0         0    50000     50    0    0    0     0       0          0
virbr0:  200000    200    0    0    0     0          0         0   150000    150    0    0    0     0       0          0
   br0:  300000    300    0    0    0     0          0         0   200000    200    0    0    0     0       0          0
br-lan:  400000    400    0    0    0     0          0         0   250000    250    0    0    0     0       0          0
 vnet0:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
`

func TestParseNetwork(t *testing.T) {
	t.Run("ubuntu multi-interface", func(t *testing.T) {
		net, err := parser.ParseNetwork(procNetDevUbuntu)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(net.Interfaces) != 2 {
			t.Fatalf("interface count = %d, want 2", len(net.Interfaces))
		}

		// eth0 (lo and docker0 filtered as virtual)
		eth0 := net.Interfaces[0]
		if eth0.Name != "eth0" {
			t.Errorf("iface 0 name = %q, want %q", eth0.Name, "eth0")
		}
		if eth0.RxBytes != 98765432100 {
			t.Errorf("eth0 rx = %d, want %d", eth0.RxBytes, 98765432100)
		}
		if eth0.TxBytes != 54321098765 {
			t.Errorf("eth0 tx = %d, want %d", eth0.TxBytes, 54321098765)
		}

		// eth1
		eth1 := net.Interfaces[1]
		if eth1.Name != "eth1" {
			t.Errorf("iface 1 name = %q, want %q", eth1.Name, "eth1")
		}
	})

	t.Run("centos single interface", func(t *testing.T) {
		net, err := parser.ParseNetwork(procNetDevCentOS)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// lo filtered as virtual; only ens3 remains
		if len(net.Interfaces) != 1 {
			t.Fatalf("interface count = %d, want 1", len(net.Interfaces))
		}

		ens3 := net.Interfaces[0]
		if ens3.Name != "ens3" {
			t.Errorf("iface 0 name = %q, want %q", ens3.Name, "ens3")
		}
		if ens3.RxBytes != 4000000000 {
			t.Errorf("ens3 rx = %d, want %d", ens3.RxBytes, 4000000000)
		}
		if ens3.TxBytes != 2000000000 {
			t.Errorf("ens3 tx = %d, want %d", ens3.TxBytes, 2000000000)
		}
	})

	t.Run("docker host filters virtual, keeps tunnels and named bridges", func(t *testing.T) {
		net, err := parser.ParseNetwork(procNetDevDocker)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Expected kept: eth0, tun0, tap0, virbr0, br0, br-lan (6 interfaces)
		// Filtered out: lo, docker0, br-29a590b46462, veth83f5d1f, vnet0
		if len(net.Interfaces) != 6 {
			t.Fatalf("interface count = %d, want 6", len(net.Interfaces))
		}

		kept := make(map[string]bool)
		for _, iface := range net.Interfaces {
			kept[iface.Name] = true
		}

		for _, name := range []string{"eth0", "tun0", "tap0", "virbr0", "br0", "br-lan"} {
			if !kept[name] {
				t.Errorf("expected %q to be kept, but it was filtered", name)
			}
		}

		for _, name := range []string{"lo", "docker0", "br-29a590b46462", "veth83f5d1f", "vnet0"} {
			if kept[name] {
				t.Errorf("expected %q to be filtered, but it was kept", name)
			}
		}
	})

	t.Run("empty input", func(t *testing.T) {
		net, err := parser.ParseNetwork("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(net.Interfaces) != 0 {
			t.Errorf("interface count = %d, want 0", len(net.Interfaces))
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
		info, err := parser.ParseSystemInfo(
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
		info, err := parser.ParseSystemInfo(
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
		info, err := parser.ParseSystemInfo(
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
		info, err := parser.ParseSystemInfo(
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
