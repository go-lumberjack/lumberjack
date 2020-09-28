**Example**

To use lumberjack with the standard library's log package, just pass it into the SetOutput function when your application starts.

Code:

```go
fileHandler, err := lumberjack.New(
    lumberjack.WithName("/var/log/myapp/foo.log"),
    lumberjack.WithMaxBytes(500*lumberjack.MB),
    lumberjack.WithMaxBackups(3),
    lumberjack.WithMaxDays(28),
    lumberjack.WithCompress(),
)
if err != nil {
    return err
}
log.SetOutput(fileHandler)
```