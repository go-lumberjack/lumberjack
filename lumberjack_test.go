package lumberjack

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// !!!NOTE!!!
//
// Running these tests in parallel will almost certainly cause sporadic (or even
// regular) failures, because they're all messing with the same global variable
// that controls the logic's mocked time.Now.  So... don't do that.

// Since all the tests uses the time to determine filenames etc, we need to
// control the wall clock as much as possible, which means having a wall clock
// that doesn't change unless we want it to.
var fakeCurrentTime = time.Now()

func fakeTime() time.Time {
	return fakeCurrentTime
}

func TestNewFile(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir("TestNewFile", t)
	defer os.RemoveAll(dir) // nolint

	filename := logFile(dir)

	l, err := New(
		WithFileName(filename))
	require.NoError(t, err)
	defer l.Close() // nolint

	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)
	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)
}

func TestOpenExisting(t *testing.T) {
	currentTime = fakeTime
	dir := makeTempDir("TestOpenExisting", t)
	defer os.RemoveAll(dir) // nolint

	filename := logFile(dir)
	data := []byte("foo!")
	err := ioutil.WriteFile(filename, data, 0600)
	require.NoError(t, err)
	existsWithContent(filename, data, t)

	l, err := New(
		WithFileName(filename))
	require.NoError(t, err)
	defer l.Close() // nolint

	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)
	existsWithContent(filename, append(data, b...), t)
	fileCount(dir, 1, t)
}

func TestWriteTooLong(t *testing.T) {
	currentTime = fakeTime
	dir := makeTempDir("TestWriteTooLong", t)
	defer os.RemoveAll(dir) // nolint

	var bytes int64 = 5
	filename := logFile(dir)
	l, err := New(
		WithFileName(filename),
		WithMaxBytes(bytes),
	)
	require.NoError(t, err)
	defer l.Close() // nolint

	b := []byte("booooooooooooooo!")
	n, err := l.Write(b)
	require.Error(t, err)
	require.Equal(t, 0, n)
	require.EqualError(t, err, fmt.Sprintf("write length %d exceeds maximum file size %d", len(b), bytes))
}

func TestMakeLogDir(t *testing.T) {
	currentTime = fakeTime
	dir := time.Now().Format("TestMakeLogDir" + backupTimeFormat)
	dir = filepath.Join(os.TempDir(), dir)
	defer os.RemoveAll(dir) // nolint

	filename := logFile(dir)
	l, err := New(
		WithFileName(filename))
	require.NoError(t, err)
	defer l.Close() // nolint

	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)
	existsWithContent(logFile(dir), b, t)
	fileCount(dir, 1, t)
}

func TestDefaultFilename(t *testing.T) {
	currentTime = fakeTime
	dir := os.TempDir()
	filename := filepath.Join(dir, filepath.Base(os.Args[0])+"-lumberjack.log")
	defer os.Remove(filename) // nolint

	l, err := New()
	require.NoError(t, err)
	defer l.Close() // nolint

	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)
	existsWithContent(filename, b, t)
}

func TestAutoRotate(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir("TestAutoRotate", t)
	defer os.RemoveAll(dir) // nolint

	filename := logFile(dir)
	l, err := New(
		WithFileName(filename),
		WithMaxBytes(10),
	)
	require.NoError(t, err)
	defer l.Close() // nolint

	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)
	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	newFakeTime()

	b2 := []byte("foooooo!")
	n, err = l.Write(b2)
	require.NoError(t, err)
	require.Equal(t, len(b2), n)
	existsWithContent(filename, b2, t)
	existsWithContent(backupFile(dir), b, t)
	fileCount(dir, 2, t)
}

func TestFirstWriteRotate(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir("TestFirstWriteRotate", t)
	defer os.RemoveAll(dir) // nolint

	filename := logFile(dir)

	start := []byte("boooooo!")
	err := ioutil.WriteFile(filename, start, 0600)
	require.NoError(t, err)

	l, err := New(
		WithFileName(filename),
		WithMaxBytes(10),
	)
	require.NoError(t, err)
	defer l.Close() // nolint

	newFakeTime()

	// this would make us rotate
	b := []byte("fooo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)
	existsWithContent(filename, b, t)
	fileCount(dir, 2, t)

	existsWithContent(backupFile(dir), start, t)
}

func TestMaxBackups(t *testing.T) { // nolint
	currentTime = fakeTime

	dir := makeTempDir("TestMaxBackups", t)
	defer os.RemoveAll(dir) // nolint

	filename := logFile(dir)
	l, err := New(
		WithFileName(filename),
		WithMaxBytes(10),
		WithMaxBackups(1),
	)
	require.NoError(t, err)
	defer l.Close() // nolint

	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)
	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	newFakeTime()

	// this will put us over the max
	b2 := []byte("foooooo!")
	n, err = l.Write(b2)
	require.NoError(t, err)
	require.Equal(t, len(b2), n)

	// this will use the new fake time
	secondFilename := backupFile(dir)
	existsWithContent(secondFilename, b, t)

	// make sure the old file still exists with the same content.
	existsWithContent(filename, b2, t)
	fileCount(dir, 2, t)

	newFakeTime()

	// this will make us rotate again
	b3 := []byte("baaaaaar!")
	n, err = l.Write(b3)
	require.NoError(t, err)
	require.Equal(t, len(b3), n)

	thirdFilename := backupFile(dir)
	existsWithContent(thirdFilename, b2, t)
	existsWithContent(filename, b3, t)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(time.Millisecond * 10)

	// should only have two files in the dir still
	fileCount(dir, 2, t)

	// second file name should still exist
	existsWithContent(thirdFilename, b2, t)

	// should have deleted the first backup
	notExist(secondFilename, t)

	// now test that we don't delete directories or non-logfile files
	newFakeTime()

	// create a file that is close to but different from the logfile name.
	// It shouldn't get caught by our deletion filters.
	notlogfile := logFile(dir) + ".foo"
	err = ioutil.WriteFile(notlogfile, []byte("data"), 0600)
	require.NoError(t, err)

	// Make a directory that exactly matches our log file filters... it still
	// shouldn't get caught by the deletion filter since it's a directory.
	notlogfiledir := backupFile(dir)
	err = os.Mkdir(notlogfiledir, 0700)
	require.NoError(t, err)

	newFakeTime()

	// this will use the new fake time
	fourthFilename := backupFile(dir)

	// Create a log file that is/was being compressed - this should
	// not be counted since both the compressed and the uncompressed
	// log files still exist.
	compLogFile := fourthFilename + compressSuffix
	err = ioutil.WriteFile(compLogFile, []byte("compress"), 0600)
	require.NoError(t, err)

	// this will make us rotate again
	b4 := []byte("baaaaaaz!")
	n, err = l.Write(b4)
	require.NoError(t, err)
	require.Equal(t, len(b4), n)

	existsWithContent(fourthFilename, b3, t)
	existsWithContent(fourthFilename+compressSuffix, []byte("compress"), t)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(time.Millisecond * 10)

	// We should have four things in the directory now - the 2 log files, the
	// not log file, and the directory
	fileCount(dir, 5, t)

	// third file name should still exist
	existsWithContent(filename, b4, t)

	existsWithContent(fourthFilename, b3, t)

	// should have deleted the first filename
	notExist(thirdFilename, t)

	// the not-a-logfile should still exist
	exists(notlogfile, t)

	// the directory
	exists(notlogfiledir, t)
}

func TestCleanupExistingBackups(t *testing.T) {
	// test that if we start with more backup files than we're supposed to have
	// in total, that extra ones get cleaned up when we rotate.
	currentTime = fakeTime

	dir := makeTempDir("TestCleanupExistingBackups", t)
	defer os.RemoveAll(dir) // nolint

	// make 3 backup files

	data := []byte("data")
	backup := backupFile(dir)
	err := ioutil.WriteFile(backup, data, 0600)
	require.NoError(t, err)

	newFakeTime()

	backup = backupFile(dir)
	err = ioutil.WriteFile(backup+compressSuffix, data, 0600)
	require.NoError(t, err)

	newFakeTime()

	backup = backupFile(dir)
	err = ioutil.WriteFile(backup, data, 0600)
	require.NoError(t, err)

	// now create a primary log file with some data
	filename := logFile(dir)
	err = ioutil.WriteFile(filename, data, 0600)
	require.NoError(t, err)

	l, err := New(
		WithFileName(filename),
		WithMaxBytes(10),
		WithMaxBackups(1),
	)
	require.NoError(t, err)
	defer l.Close() // nolint

	newFakeTime()

	b2 := []byte("foooooo!")
	n, err := l.Write(b2)
	require.NoError(t, err)
	require.Equal(t, len(b2), n)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(time.Millisecond * 10)

	// now we should only have 2 files left - the primary and one backup
	fileCount(dir, 2, t)
}

func TestMaxAge(t *testing.T) { // nolint
	currentTime = fakeTime

	dir := makeTempDir("TestMaxAge", t)
	defer os.RemoveAll(dir) // nolint

	filename := logFile(dir)
	l, err := New(
		WithFileName(filename),
		WithMaxBytes(10),
		WithMaxDays(1),
	)
	require.NoError(t, err)
	defer l.Close() // nolint

	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)

	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	// two days later
	newFakeTime()

	b2 := []byte("foooooo!")
	n, err = l.Write(b2)
	require.NoError(t, err)
	require.Equal(t, len(b2), n)
	existsWithContent(backupFile(dir), b, t)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// We should still have 2 log files, since the most recent backup was just
	// created.
	fileCount(dir, 2, t)

	existsWithContent(filename, b2, t)

	// we should have deleted the old file due to being too old
	existsWithContent(backupFile(dir), b, t)

	// two days later
	newFakeTime()

	b3 := []byte("baaaaar!")
	n, err = l.Write(b3)
	require.NoError(t, err)
	require.Equal(t, len(b3), n)
	existsWithContent(backupFile(dir), b2, t)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// We should have 2 log files - the main log file, and the most recent
	// backup.  The earlier backup is past the cutoff and should be gone.
	fileCount(dir, 2, t)

	existsWithContent(filename, b3, t)

	// we should have deleted the old file due to being too old
	existsWithContent(backupFile(dir), b2, t)
}

func TestOldLogFiles(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir("TestOldLogFiles", t)
	defer os.RemoveAll(dir) // nolint

	filename := logFile(dir)
	data := []byte("data")
	err := ioutil.WriteFile(filename, data, 07)
	require.NoError(t, err)

	// This gives us a time with the same precision as the time we get from the
	// timestamp in the name.
	t1, err := time.Parse(backupTimeFormat, fakeTime().UTC().Format(backupTimeFormat))
	require.NoError(t, err)

	backup := backupFile(dir)
	err = ioutil.WriteFile(backup, data, 07)
	require.NoError(t, err)

	newFakeTime()

	t2, err := time.Parse(backupTimeFormat, fakeTime().UTC().Format(backupTimeFormat))
	require.NoError(t, err)

	backup2 := backupFile(dir)
	err = ioutil.WriteFile(backup2, data, 07)
	require.NoError(t, err)

	l := &loggerOption{filename: filename}
	files, err := l.oldLogFiles()
	require.NoError(t, err)
	require.Equal(t, 2, len(files))

	// should be sorted by newest file first, which would be t2
	require.Equal(t, t2, files[0].timestamp)
	require.Equal(t, t1, files[1].timestamp)
}

func TestTimeFromName(t *testing.T) {
	l := &loggerOption{filename: "/var/log/myfoo/foo.log"}
	prefix, ext := l.prefixAndExt()

	tests := []struct {
		filename string
		want     time.Time
		wantErr  bool
	}{
		{"foo-2014-05-04T14-44-33.555.log", time.Date(2014, 5, 4, 14, 44, 33, 555000000, time.UTC), false},
		{"foo-2014-05-04T14-44-33.555", time.Time{}, true},
		{"2014-05-04T14-44-33.555.log", time.Time{}, true},
		{"foo.log", time.Time{}, true},
	}

	for _, test := range tests {
		got, err := l.timeFromName(test.filename, prefix, ext)
		require.Equal(t, test.want, got)
		require.Equal(t, test.wantErr, err != nil)
	}
}

func TestLocalTime(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir("TestLocalTime", t)
	defer os.RemoveAll(dir) // nolint

	l, err := New(
		WithFileName(logFile(dir)),
		WithMaxBytes(10),
		WithLocalTime(),
	)
	require.NoError(t, err)
	defer l.Close() // nolint

	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)

	b2 := []byte("fooooooo!")
	n, err = l.Write(b2)
	require.NoError(t, err)
	require.Equal(t, len(b2), n)

	existsWithContent(logFile(dir), b2, t)
	existsWithContent(backupFileLocal(dir), b, t)
}

func TestRotate(t *testing.T) {
	currentTime = fakeTime
	dir := makeTempDir("TestRotate", t)
	defer os.RemoveAll(dir) // nolint

	filename := logFile(dir)

	l, err := New(
		WithFileName(filename),
		WithMaxBytes(100),
		WithMaxBackups(1),
	)
	require.NoError(t, err)
	defer l.Close() // nolint

	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)

	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	newFakeTime()

	err = l.Rotate()
	require.NoError(t, err)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	filename2 := backupFile(dir)
	existsWithContent(filename2, b, t)
	existsWithContent(filename, []byte{}, t)
	fileCount(dir, 2, t)

	newFakeTime()

	err = l.Rotate()
	require.NoError(t, err)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	filename3 := backupFile(dir)
	existsWithContent(filename3, []byte{}, t)
	existsWithContent(filename, []byte{}, t)
	fileCount(dir, 2, t)

	b2 := []byte("foooooo!")
	n, err = l.Write(b2)
	require.NoError(t, err)
	require.Equal(t, len(b2), n)

	// this will use the new fake time
	existsWithContent(filename, b2, t)
}

func TestCompressOnRotate(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir("TestCompressOnRotate", t)
	defer os.RemoveAll(dir) // nolint

	filename := logFile(dir)
	l, err := New(
		WithFileName(filename),
		WithMaxBytes(10),
		WithCompress(),
	)
	require.NoError(t, err)
	defer l.Close() // nolint

	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)

	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	newFakeTime()

	err = l.Rotate()
	require.NoError(t, err)

	// the old logfile should be moved aside and the main logfile should have
	// nothing in it.
	existsWithContent(filename, []byte{}, t)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(300 * time.Millisecond)

	// a compressed version of the log file should now exist and the original
	// should have been removed.
	bc := new(bytes.Buffer)
	gz := gzip.NewWriter(bc)
	_, err = gz.Write(b)
	require.NoError(t, err)

	err = gz.Close()
	require.NoError(t, err)

	existsWithContent(backupFile(dir)+compressSuffix, bc.Bytes(), t)
	notExist(backupFile(dir), t)

	fileCount(dir, 2, t)
}

func TestCompressOnResume(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir("TestCompressOnResume", t)
	defer os.RemoveAll(dir) // nolint

	// Create a backup file and empty "compressed" file.
	filename2 := backupFile(dir)
	b := []byte("foo!")
	err := ioutil.WriteFile(filename2, b, 0600)
	require.NoError(t, err)

	err = ioutil.WriteFile(filename2+compressSuffix, []byte{}, 0600)
	require.NoError(t, err)

	filename := logFile(dir)
	l, err := New(
		WithFileName(filename),
		WithMaxBytes(10),
		WithCompress(),
	)
	require.NoError(t, err)
	defer l.Close() // nolint

	newFakeTime()

	b2 := []byte("boo!")
	n, err := l.Write(b2)
	require.NoError(t, err)
	require.Equal(t, len(b2), n)
	existsWithContent(filename, b2, t)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(300 * time.Millisecond)

	// The write should have started the compression - a compressed version of
	// the log file should now exist and the original should have been removed.
	bc := new(bytes.Buffer)
	gz := gzip.NewWriter(bc)
	_, err = gz.Write(b)
	require.NoError(t, err)

	err = gz.Close()
	require.NoError(t, err)
	existsWithContent(filename2+compressSuffix, bc.Bytes(), t)
	notExist(filename2, t)

	fileCount(dir, 2, t)
}

func TestGoRoutinesNotLeaked(t *testing.T) {
	dir := makeTempDir("TestGoRoutinesNotLeaked", t)
	defer os.RemoveAll(dir) // nolint

	numGoRoutinesBefore := pprof.Lookup("goroutine").Count()
	filename := logFile(dir)
	for i := 0; i < 25; i++ {
		func() {
			l, err := New(
				WithFileName(filename))
			require.NoError(t, err)
			defer l.Close() // nolint

			b := []byte("boo!")
			_, err = l.Write(b)
			require.NoError(t, err)
		}()
	}
	time.Sleep(1 * time.Millisecond)
	numGoRoutinesAfter := pprof.Lookup("goroutine").Count()

	// all loggers have been closed, so number of goroutines should not have increased
	require.Equal(t, numGoRoutinesBefore, numGoRoutinesAfter)
}

func TestBufferSize(t *testing.T) {
	dir := makeTempDir("TestBufferSize", t)
	defer os.RemoveAll(dir) // nolint

	filename := logFile(dir)
	l, err := New(
		WithFileName(filename),
		WithBufferSize(10), // set buffer 10 bytes
	)
	require.NoError(t, err)
	defer l.Close() // nolint

	b := []byte("fooo!") // append 5 bytes
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)
	existsWithContent(filename, []byte{}, t)

	b2 := []byte("fooooo!") // append 5 bytes
	_, err = l.Write(b2)
	require.NoError(t, err)
	existsWithContent(filename, []byte("fooo!foooo"), t)

	err = l.Flush()
	require.NoError(t, err)
	existsWithContent(filename, []byte("fooo!fooooo!"), t)
}

func TestWriteInCloseFile(t *testing.T) {
	dir := makeTempDir("TestWriteInCloseFile", t)
	defer os.RemoveAll(dir) // nolint

	filename := logFile(dir)
	l, err := New(
		WithFileName(filename))
	require.NoError(t, err)

	b := []byte("fooo!") // append 5 bytes
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)
	existsWithContent(filename, b, t)

	err = l.Close()
	require.NoError(t, err)

	n, err = l.Write(b)
	require.Error(t, err)
	require.Equal(t, 0, n)
	require.EqualError(t, err, "file close")
}

func TestReWrite(t *testing.T) {
	dir := makeTempDir("TestReWrite", t)
	defer os.RemoveAll(dir) // nolint

	filename := logFile(dir)
	l, err := New(
		WithFileName(filename),
		WithMaxBytes(10),
		WithReWrite()) // set 10 bytes max file
	require.NoError(t, err)

	b := []byte("fooo!") // append 5 bytes
	n, err := l.Write(b)
	require.NoError(t, err)
	require.Equal(t, len(b), n)
	existsWithContent(filename, b, t)

	b2 := []byte("booo!brrr!") // append 10 bytes
	n, err = l.Write(b2)
	require.NoError(t, err)
	require.Equal(t, len(b2), n)
	fileCount(dir, 1, t)
}

// makeTempDir creates a file with a semi-unique name in the OS temp directory.
// It should be based on the name of the test, to keep parallel tests from
// colliding, and must be cleaned up after the test is finished.
func makeTempDir(name string, t testing.TB) string {
	dir := time.Now().Format(name + backupTimeFormat)
	dir = filepath.Join(os.TempDir(), dir)
	err := os.Mkdir(dir, 0700)
	require.NoError(t, err)
	return dir
}

// existsWithContent checks that the given file exists and has the correct content.
func existsWithContent(path string, content []byte, t testing.TB) {
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, info.Size(), int64(len(content)))

	b, err := ioutil.ReadFile(path) // nolint
	require.NoError(t, err)
	require.Equal(t, b, content)
}

// logFile returns the log file name in the given directory for the current fake
// time.
func logFile(dir string) string {
	return filepath.Join(dir, "foobar.log")
}

func backupFile(dir string) string {
	return filepath.Join(dir, "foobar-"+fakeTime().UTC().Format(backupTimeFormat)+".log")
}

func backupFileLocal(dir string) string {
	return filepath.Join(dir, "foobar-"+fakeTime().Format(backupTimeFormat)+".log")
}

// fileCount checks that the number of files in the directory is exp.
func fileCount(dir string, exp int, t testing.TB) {
	files, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	// Make sure no other files were created.
	require.Equal(t, exp, len(files))
}

// newFakeTime sets the fake "current time" to two days later.
func newFakeTime() {
	fakeCurrentTime = fakeCurrentTime.Add(time.Hour * 24 * 2)
}

func notExist(path string, t testing.TB) {
	_, err := os.Stat(path)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}

func exists(path string, t testing.TB) {
	_, err := os.Stat(path)
	require.NoError(t, err)
}
