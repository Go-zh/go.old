// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filepath_test

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

func init() {
	tmpdir, err := ioutil.TempDir("", "symtest")
	if err != nil {
		panic("failed to create temp directory: " + err.Error())
	}
	defer os.RemoveAll(tmpdir)

	err = os.Symlink("target", filepath.Join(tmpdir, "symlink"))
	if err == nil {
		return
	}

	err = err.(*os.LinkError).Err
	switch err {
	case syscall.EWINDOWS, syscall.ERROR_PRIVILEGE_NOT_HELD:
		supportsSymlinks = false
	}
}

func TestWinSplitListTestsAreValid(t *testing.T) {
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		t.Fatal("%ComSpec% must be set")
	}

	for ti, tt := range winsplitlisttests {
		testWinSplitListTestIsValid(t, ti, tt, comspec)
	}
}

func testWinSplitListTestIsValid(t *testing.T, ti int, tt SplitListTest,
	comspec string) {

	const (
		cmdfile             = `printdir.cmd`
		perm    os.FileMode = 0700
	)

	tmp, err := ioutil.TempDir("", "testWinSplitListTestIsValid")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	defer os.RemoveAll(tmp)

	for i, d := range tt.result {
		if d == "" {
			continue
		}
		if cd := filepath.Clean(d); filepath.VolumeName(cd) != "" ||
			cd[0] == '\\' || cd == ".." || (len(cd) >= 3 && cd[0:3] == `..\`) {
			t.Errorf("%d,%d: %#q refers outside working directory", ti, i, d)
			return
		}
		dd := filepath.Join(tmp, d)
		if _, err := os.Stat(dd); err == nil {
			t.Errorf("%d,%d: %#q already exists", ti, i, d)
			return
		}
		if err = os.MkdirAll(dd, perm); err != nil {
			t.Errorf("%d,%d: MkdirAll(%#q) failed: %v", ti, i, dd, err)
			return
		}
		fn, data := filepath.Join(dd, cmdfile), []byte("@echo "+d+"\r\n")
		if err = ioutil.WriteFile(fn, data, perm); err != nil {
			t.Errorf("%d,%d: WriteFile(%#q) failed: %v", ti, i, fn, err)
			return
		}
	}

	for i, d := range tt.result {
		if d == "" {
			continue
		}
		exp := []byte(d + "\r\n")
		cmd := &exec.Cmd{
			Path: comspec,
			Args: []string{`/c`, cmdfile},
			Env:  []string{`Path=` + tt.list},
			Dir:  tmp,
		}
		out, err := cmd.CombinedOutput()
		switch {
		case err != nil:
			t.Errorf("%d,%d: execution error %v\n%q", ti, i, err, out)
			return
		case !reflect.DeepEqual(out, exp):
			t.Errorf("%d,%d: expected %#q, got %#q", ti, i, exp, out)
			return
		default:
			// unshadow cmdfile in next directory
			err = os.Remove(filepath.Join(tmp, d, cmdfile))
			if err != nil {
				t.Fatalf("Remove test command failed: %v", err)
			}
		}
	}
}

// TestEvalSymlinksCanonicalNames verify that EvalSymlinks
// returns "canonical" path names on windows.
func TestEvalSymlinksCanonicalNames(t *testing.T) {
	tmp, err := ioutil.TempDir("", "evalsymlinkcanonical")
	if err != nil {
		t.Fatal("creating temp dir:", err)
	}
	defer os.RemoveAll(tmp)

	// ioutil.TempDir might return "non-canonical" name.
	cTmpName, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Errorf("EvalSymlinks(%q) error: %v", tmp, err)
	}

	dirs := []string{
		"test",
		"test/dir",
		"testing_long_dir",
		"TEST2",
	}

	for _, d := range dirs {
		dir := filepath.Join(cTmpName, d)
		err := os.Mkdir(dir, 0755)
		if err != nil {
			t.Fatal(err)
		}
		cname, err := filepath.EvalSymlinks(dir)
		if err != nil {
			t.Errorf("EvalSymlinks(%q) error: %v", dir, err)
			continue
		}
		if dir != cname {
			t.Errorf("EvalSymlinks(%q) returns %q, but should return %q", dir, cname, dir)
			continue
		}
		// test non-canonical names
		test := strings.ToUpper(dir)
		p, err := filepath.EvalSymlinks(test)
		if err != nil {
			t.Errorf("EvalSymlinks(%q) error: %v", test, err)
			continue
		}
		if p != cname {
			t.Errorf("EvalSymlinks(%q) returns %q, but should return %q", test, p, cname)
			continue
		}
		// another test
		test = strings.ToLower(dir)
		p, err = filepath.EvalSymlinks(test)
		if err != nil {
			t.Errorf("EvalSymlinks(%q) error: %v", test, err)
			continue
		}
		if p != cname {
			t.Errorf("EvalSymlinks(%q) returns %q, but should return %q", test, p, cname)
			continue
		}
	}
}

// checkVolume8dot3Setting runs "fsutil 8dot3name query c:" command
// (where c: is vol parameter) to discover "8dot3 name creation state".
// The state is combination of 2 flags. The global flag controls if it
// is per volume or global setting:
//   0 - Enable 8dot3 name creation on all volumes on the system
//   1 - Disable 8dot3 name creation on all volumes on the system
//   2 - Set 8dot3 name creation on a per volume basis
//   3 - Disable 8dot3 name creation on all volumes except the system volume
// If global flag is set to 2, then per-volume flag needs to be examined:
//   0 - Enable 8dot3 name creation on this volume
//   1 - Disable 8dot3 name creation on this volume
// checkVolume8dot3Setting verifies that "8dot3 name creation" flags
// are set to 2 and 0, if enabled parameter is true, or 2 and 1, if enabled
// is false. Otherwise checkVolume8dot3Setting returns error.
func checkVolume8dot3Setting(vol string, enabled bool) error {
	// It appears, on some systems "fsutil 8dot3name query ..." command always
	// exits with error. Ignore exit code, and look at fsutil output instead.
	out, _ := exec.Command("fsutil", "8dot3name", "query", vol).CombinedOutput()
	// Check that system has "Volume level setting" set.
	expected := "The registry state of NtfsDisable8dot3NameCreation is 2, the default (Volume level setting)"
	if !strings.Contains(string(out), expected) {
		// Windows 10 version of fsutil has different output message.
		expectedWindow10 := "The registry state is: 2 (Per volume setting - the default)"
		if !strings.Contains(string(out), expectedWindow10) {
			return fmt.Errorf("fsutil output should contain %q, but is %q", expected, string(out))
		}
	}
	// Now check the volume setting.
	expected = "Based on the above two settings, 8dot3 name creation is %s on %s"
	if enabled {
		expected = fmt.Sprintf(expected, "enabled", vol)
	} else {
		expected = fmt.Sprintf(expected, "disabled", vol)
	}
	if !strings.Contains(string(out), expected) {
		return fmt.Errorf("unexpected fsutil output: %q", string(out))
	}
	return nil
}

func setVolume8dot3Setting(vol string, enabled bool) error {
	cmd := []string{"fsutil", "8dot3name", "set", vol}
	if enabled {
		cmd = append(cmd, "0")
	} else {
		cmd = append(cmd, "1")
	}
	// It appears, on some systems "fsutil 8dot3name set ..." command always
	// exits with error. Ignore exit code, and look at fsutil output instead.
	out, _ := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if string(out) != "\r\nSuccessfully set 8dot3name behavior.\r\n" {
		// Windows 10 version of fsutil has different output message.
		expectedWindow10 := "Successfully %s 8dot3name generation on %s\r\n"
		if enabled {
			expectedWindow10 = fmt.Sprintf(expectedWindow10, "enabled", vol)
		} else {
			expectedWindow10 = fmt.Sprintf(expectedWindow10, "disabled", vol)
		}
		if string(out) != expectedWindow10 {
			return fmt.Errorf("%v command failed: %q", cmd, string(out))
		}
	}
	return nil
}

var runFSModifyTests = flag.Bool("run_fs_modify_tests", false, "run tests which modify filesystem parameters")

// This test assumes registry state of NtfsDisable8dot3NameCreation is 2,
// the default (Volume level setting).
func TestEvalSymlinksCanonicalNamesWith8dot3Disabled(t *testing.T) {
	if !*runFSModifyTests {
		t.Skip("skipping test that modifies file system setting; enable with -run_fs_modify_tests")
	}
	tempVol := filepath.VolumeName(os.TempDir())
	if len(tempVol) != 2 {
		t.Fatalf("unexpected temp volume name %q", tempVol)
	}

	err := checkVolume8dot3Setting(tempVol, true)
	if err != nil {
		t.Fatal(err)
	}
	err = setVolume8dot3Setting(tempVol, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := setVolume8dot3Setting(tempVol, true)
		if err != nil {
			t.Fatal(err)
		}
		err = checkVolume8dot3Setting(tempVol, true)
		if err != nil {
			t.Fatal(err)
		}
	}()
	err = checkVolume8dot3Setting(tempVol, false)
	if err != nil {
		t.Fatal(err)
	}
	TestEvalSymlinksCanonicalNames(t)
}

func TestToNorm(t *testing.T) {
	stubBase := func(path string) (string, error) {
		vol := filepath.VolumeName(path)
		path = path[len(vol):]

		if strings.Contains(path, "/") {
			return "", fmt.Errorf("invalid path is given to base: %s", vol+path)
		}

		if path == "" || path == "." || path == `\` {
			return "", fmt.Errorf("invalid path is given to base: %s", vol+path)
		}

		i := strings.LastIndexByte(path, filepath.Separator)
		if i == len(path)-1 { // trailing '\' is invalid
			return "", fmt.Errorf("invalid path is given to base: %s", vol+path)
		}
		if i == -1 {
			return strings.ToUpper(path), nil
		}

		return strings.ToUpper(path[i+1:]), nil
	}

	// On this test, toNorm should be same as string.ToUpper(filepath.Clean(path)) except empty string.
	tests := []struct {
		arg  string
		want string
	}{
		{"", ""},
		{".", "."},
		{"./foo/bar", `FOO\BAR`},
		{"/", `\`},
		{"/foo/bar", `\FOO\BAR`},
		{"/foo/bar/baz/qux", `\FOO\BAR\BAZ\QUX`},
		{"foo/bar", `FOO\BAR`},
		{"C:/foo/bar", `C:\FOO\BAR`},
		{"C:foo/bar", `C:FOO\BAR`},
		{"c:/foo/bar", `C:\FOO\BAR`},
		{"C:/foo/bar", `C:\FOO\BAR`},
		{"C:/foo/bar/", `C:\FOO\BAR`},
		{`C:\foo\bar`, `C:\FOO\BAR`},
		{`C:\foo/bar\`, `C:\FOO\BAR`},
		{"C:/ふー/バー", `C:\ふー\バー`},
	}

	for _, test := range tests {
		got, err := filepath.ToNorm(test.arg, stubBase)
		if err != nil {
			t.Errorf("unexpected toNorm error, arg: %s, err: %v\n", test.arg, err)
		} else if got != test.want {
			t.Errorf("toNorm error, arg: %s, want: %s, got: %s\n", test.arg, test.want, got)
		}
	}
}
