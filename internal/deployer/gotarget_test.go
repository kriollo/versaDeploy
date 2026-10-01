package deployer

import "testing"

func TestGoarchesFor(t *testing.T) {
	cases := map[string]string{ // uname -m -> first (native) GOARCH
		"x86_64": "amd64", "i686": "386", "i386": "386", "aarch64": "arm64",
		"armv7l": "arm", "armv6l": "arm", "armv8l": "arm", "ppc64le": "ppc64le",
		"s390x": "s390x", "riscv64": "riscv64", "loongarch64": "loong64", "mips64": "mips64",
	}
	for machine, want := range cases {
		if got := goarchesFor(machine); len(got) == 0 || got[0] != want {
			t.Errorf("goarchesFor(%q) = %v, want %s first", machine, got, want)
		}
	}
	if !contains(goarchesFor("x86_64"), "386") {
		t.Error("386 binaries run on x86_64")
	}
	if goarchesFor("sparc64") != nil {
		t.Error("unknown machine should return nil")
	}
	if maxGOARM("armv6l") != 6 || maxGOARM("armv7l") != 0 || maxGOARM("aarch64") != 0 {
		t.Error("maxGOARM")
	}
}

func TestVersionAtLeast(t *testing.T) {
	cases := []struct {
		v    string
		want []int
		ok   bool
	}{
		{"2.6.18-419.el5", []int{3, 2}, false},
		{"3.10.0-1160.el7.x86_64", []int{3, 2}, true},
		{"3.2.0", []int{3, 2}, true},
		{"6.8.0-45-generic", []int{3, 2}, true},
		{"go1.26.1", []int{1, 24}, true},
		{"go1.23.12", []int{1, 24}, false},
	}
	for _, c := range cases {
		if got := versionAtLeast(c.v, c.want...); got != c.ok {
			t.Errorf("versionAtLeast(%q, %v) = %v", c.v, c.want, got)
		}
	}
}
