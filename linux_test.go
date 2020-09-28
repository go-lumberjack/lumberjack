// +build linux

package lumberjack

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMaintainMode(t *testing.T) {
	currentTime = fakeTime
	dir := makeTempDir("TestMaintainMode", t)
	defer os.RemoveAll(dir) // nolint

	filename := logFile(dir)

	mode := os.FileMode(0600)
	file, err := os.OpenFile(filepath.Clean(filename), os.O_CREATE|os.O_RDWR, mode) // nolint
	require.NoError(t, err)
	file.Close() // nolint

	l, err := New(
		WithName(filename),
		WithMaxBackups(1),
		WithMaxBytes(100),
	)
	require.NoError(t, err)
	defer l.Close()

	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)

	newFakeTime()

	err = l.Rotate()
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	filename2 := backupFile(dir)
	info, err := os.Stat(filename)
	require.NoError(t, err)
	info2, err := os.Stat(filename2)
	require.NoError(t, err)
	require.Equal(t, mode, info.Mode())
	require.Equal(t, mode, info2.Mode())
}

func TestMaintainOwner(t *testing.T) {
	fakeFS := newFakeFS()
	Chown = fakeFS.Chown
	Stat = fakeFS.Stat
	defer func() {
		Chown = os.Chown
		Stat = os.Stat
	}()
	currentTime = fakeTime
	dir := makeTempDir("TestMaintainOwner", t)
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0600) // nolint
	require.NoError(t, err)
	f.Close()

	l, err := New(
		WithName(filename),
		WithMaxBackups(1),
		WithMaxBytes(100),
	)
	require.NoError(t, err)
	defer l.Close()

	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)

	newFakeTime()

	err = l.Rotate()
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	require.Equal(t, 555, fakeFS.files[filename].uid)
	require.Equal(t, 666, fakeFS.files[filename].gid)
}

func TestCompressMaintainMode(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir("TestCompressMaintainMode", t)
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	mode := os.FileMode(0600)
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, mode) // nolint
	require.NoError(t, err)
	f.Close()

	l, err := New(
		WithName(filename),
		WithMaxBackups(1),
		WithMaxBytes(100),
		WithCompress(),
	)
	require.NoError(t, err)
	defer l.Close()

	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)

	newFakeTime()

	err = l.Rotate()
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// a compressed version of the log file should now exist with the correct
	// mode.
	filename2 := backupFile(dir)
	info, err := os.Stat(filename)
	require.NoError(t, err)
	info2, err := os.Stat(filename2 + compressSuffix)
	require.NoError(t, err)
	require.Equal(t, mode, info.Mode())
	require.Equal(t, mode, info2.Mode())
}

func TestCompressMaintainOwner(t *testing.T) {
	fakeFS := newFakeFS()
	Chown = fakeFS.Chown
	Stat = fakeFS.Stat
	defer func() {
		Chown = os.Chown
		Stat = os.Stat
	}()
	currentTime = fakeTime
	dir := makeTempDir("TestCompressMaintainOwner", t)
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0600) // nolint
	require.NoError(t, err)
	f.Close()

	l, err := New(
		WithName(filename),
		WithMaxBackups(1),
		WithMaxBytes(100),
		WithCompress(),
	)
	require.NoError(t, err)
	defer l.Close()

	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)

	newFakeTime()

	err = l.Rotate()
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// a compressed version of the log file should now exist with the correct
	// owner.
	filename2 := backupFile(dir)
	require.Equal(t, 555, fakeFS.files[filename2+compressSuffix].uid)
	require.Equal(t, 666, fakeFS.files[filename2+compressSuffix].gid)
}

type fakeFile struct {
	uid int
	gid int
}

type fakeFS struct {
	files map[string]fakeFile
}

func newFakeFS() *fakeFS {
	return &fakeFS{files: make(map[string]fakeFile)}
}

func (fs *fakeFS) Chown(name string, uid, gid int) error {
	fs.files[name] = fakeFile{uid: uid, gid: gid}
	return nil
}

func (fs *fakeFS) Stat(name string) (os.FileInfo, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	stat := info.Sys().(*syscall.Stat_t)
	stat.Uid = 555
	stat.Gid = 666
	return info, nil
}
