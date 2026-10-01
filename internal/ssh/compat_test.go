package ssh

import (
	"os/exec"
	"testing"
)

func TestParseDfAvailable(t *testing.T) {
	cases := map[string]int64{
		// df -Pk on RHEL5 with a long LVM device name: stays on one line
		"Filesystem         1024-blocks      Used Available Capacity Mounted on\n" +
			"/dev/mapper/VolGroup00-LogVol00  25396228  12698114  12698114      50% /\n": 12698114 * 1024,
		// modern coreutils
		"Filesystem     1024-blocks   Used Available Capacity Mounted on\n/dev/sda1 1000 400 600 40% /var\n": 600 * 1024,
	}
	for out, want := range cases {
		got, err := parseDfAvailable(out)
		if err != nil || got != want {
			t.Errorf("parseDfAvailable(%q) = %d, %v; want %d", out, got, err, want)
		}
	}

	// Wrapped (non -P) output must be rejected, never read "50%" as 50 bytes
	wrapped := "Filesystem 1K-blocks Used Available Use% Mounted on\n/dev/mapper/VolGroup00-LogVol00\n 25396228 12698114 12698114 50% /\n"
	if got, err := parseDfAvailable(wrapped); err == nil {
		t.Errorf("wrapped df output accepted as %d", got)
	}
}

func TestShWrapQuoting(t *testing.T) {
	cmd := `echo "a b" 'it'"'"'s' $((1+1)) | tr -d x`
	direct, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		t.Skip("no /bin/sh")
	}
	wrapped, err := exec.Command("/bin/sh", "-c", shWrap(cmd)).Output()
	if err != nil || string(wrapped) != string(direct) {
		t.Errorf("shWrap changed behavior: %q vs %q (%v)", wrapped, direct, err)
	}
}
