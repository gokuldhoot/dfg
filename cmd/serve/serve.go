package move

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

// Globals
var (
	bindAddress = "localhost:8080"
	readWrite   = false
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
	commandDefintion.Flags().StringVarP(&bindAddress, "bind", "", bindAddress, "IPaddress:Port to bind server to.")
	commandDefintion.Flags().BoolVarP(&readWrite, "rw", "", readWrite, "Serve in read/write mode.")
}

var commandDefintion = &cobra.Command{
	Use:   "serve remote:path",
	Short: `Serve the remote over HTTP.`,
	Long: `
This serves the remote over HTTP.  This can be viewed in a web browser
or you can make a remote of type FIXME to talk to it.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)
		cmd.Run(true, true, command, func() error {
			s := server{
				f:           f,
				bindAddress: bindAddress,
				readWrite:   readWrite,
			}
			s.serve()
			return nil
		})
	},
}

type server struct {
	f           fs.Fs
	bindAddress string
	readWrite   bool
}

func (s *server) serve() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handler)
	// FIXME make a transport?
	httpServer := &http.Server{
		Addr:           bindAddress,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	fs.Logf(s.f, "Serving on http://%s/", bindAddress)
	log.Fatal(httpServer.ListenAndServe())
}

func (s *server) handler(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	isDir := strings.HasSuffix(urlPath, "/")
	remote := strings.Trim(urlPath, "/")
	if isDir {
		s.serveDir(w, r, remote)
	} else {
		s.serveFile(w, r, remote)
	}
}

type entry struct {
	remote string
	URL    string
	Leaf   string
}

type entries []entry

func (a entries) Len() int           { return len(a) }
func (a entries) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a entries) Less(i, j int) bool { return a[i].remote < a[j].remote }

// FIXME add title
var indexPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>{{ .Title }}</title>
</head>
<body>
{{ range $i := .Entries }}<a href="{{ $i.URL }}">{{ $i.Leaf }}</a><br />
{{ end }}
</body>
</html>`

var indexTemplate = template.Must(template.New("index").Parse(indexPage))

type indexData struct {
	Title   string
	Entries entries
}

func (s *server) serveDir(w http.ResponseWriter, r *http.Request, dirRemote string) {
	dirEntries, err := fs.ListDirSorted(s.f, false, dirRemote)
	if err == fs.ErrorDirNotFound {
		http.Error(w, "Directory not found", http.StatusNotFound)
		return
	} else if err != nil {
		fs.Errorf(dirRemote, "Failed to list directory: %v", err)
		http.Error(w, "Failed to list directory.", http.StatusInternalServerError)
		return
	}

	var out entries
	for _, o := range dirEntries {
		remote := strings.Trim(o.Remote(), "/")
		leaf := path.Base(remote)
		urlRemote := leaf
		if _, ok := o.(*fs.Dir); ok {
			leaf += "/"
			urlRemote += "/"
		}
		out = append(out, entry{remote: remote, URL: urlRemote, Leaf: leaf})
	}

	err = indexTemplate.Execute(w, indexData{
		Entries: out,
		Title:   fmt.Sprintf("Directory: %s", dirRemote),
	})
	if err != nil {
		fs.Errorf(dirRemote, "Failed to render template: %v", err)
		http.Error(w, "Failed to render template.", http.StatusInternalServerError)
		return
	} // if dirRemote != "" {
	// 	fmt.Fprintf(w, "\n")
	// }
	// for _, item := range out {
	// 	fmt.Fprintf(w, "<a href=\"%s\">%s</a><br />\n", item.url, item.html)
	// }
}

func (s *server) serveFile(w http.ResponseWriter, r *http.Request, remote string) {
	// FIXME could cache the directories and objects...
	obj, err := s.f.NewObject(remote)
	if err == fs.ErrorObjectNotFound {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	} else if err != nil {
		fs.Errorf(remote, "Failed to find file: %v", err)
		http.Error(w, "Failed to find file.", http.StatusInternalServerError)
		return
	}

	// Check the object is included in the filters
	if !fs.Config.Filter.IncludeObject(obj) {
		fs.Errorf(remote, "Attempt to read excluded object")
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Copy the contents of the object to the output
	in, err := obj.Open()
	if err != nil {
		fs.Errorf(remote, "Failed to open file: %v", err)
		http.Error(w, "Failed to open file.", http.StatusInternalServerError)
		return
	}
	defer func() {
		err := in.Close()
		if err != nil {
			fs.Errorf(remote, "Failed to close file: %v", err)
		}
	}()

	mimeType := fs.MimeType(obj)
	if mimeType == "application/octet-stream" && path.Ext(remote) == "" {
		// Leave header blank so http server guesses
	} else {
		w.Header().Set("Content-Type", mimeType)
	}
	_, err = io.Copy(w, in)
	if err != nil {
		fs.Errorf(remote, "Failed to write file: %v", err)
	}
}
