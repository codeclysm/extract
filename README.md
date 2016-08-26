Extract
=====================

    import "gopkg.in/codeclysm/extract.v1"

Package extract allows to extract archives in zip, tar.gz or tar.bz2 formats
easily.

Most of the time you'll just need to call the proper function with a buffer and
a destination:

```go
data, _ := ioutil.ReadFile("path/to/file.tar.bz2")
buffer := bytes.NewBuffer(data)
extract.TarBz2(data, "/path/where/to/extract", nil)
```

Sometimes you'll want a bit more control over the files, such as extracting a
subfolder of the archive. In this cases you can specify a renamer func that will
change the path for every file:

```go
var shift = func(path string) string {
    parts := strings.Split(path, string(filepath.Separator))
    parts = parts[1:]
    return strings.Join(parts, string(filepath.Separator))
}
extract.TarBz2(data, "/path/where/to/extract", shift)
```



Functions
---------


```go
func Tar(body io.Reader, location string, rename Renamer) error
```

Tar extracts a .tar archived stream of data in the specified location. It
accepts a rename function to handle the names of the files (see the example)


```go
func TarBz2(body io.Reader, location string, rename Renamer) error
```

TarBz2 extracts a .tar.bz2 archived stream of data in the specified location. It
accepts a rename function to handle the names of the files (see the example)


```go
func TarGz(body io.Reader, location string, rename Renamer) error
```

TarGz extracts a .tar.gz archived stream of data in the specified location. It
accepts a rename function to handle the names of the files (see the example)


```go
func Zip(body io.Reader, location string, rename Renamer) error
```

Zip extracts a .zip archived stream of data in the specified location. It
accepts a rename function to handle the names of the files (see the example).

Types
-----


```go
type Renamer func(string) string
```
Renamer is a function that can be used to rename the files when you're
extracting them. For example you may want to only extract files with a certain
pattern. If you return an empty string they won't be extracted.
