package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/adnanh/webhook/hook"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"

	fsnotify "gopkg.in/fsnotify.v1"
)

const (
	version = "2.3.2"
)

var (
	ip             = flag.String("ip", "", "ip the webhook should serve hooks on")
	port           = flag.Int("port", 9000, "port the webhook should serve hooks on")
	verbose        = flag.Bool("verbose", false, "show verbose output")
	hotReload      = flag.Bool("hotreload", false, "watch hooks file for changes and reload them automatically")
	hooksFilePath  = flag.String("hooks", "hooks.json", "path to the json file containing defined hooks the webhook should serve")
	hooksURLPrefix = flag.String("urlprefix", "hooks", "url prefix to use for served hooks (protocol://yourserver:port/PREFIX/:hook-id)")
	secure         = flag.Bool("secure", false, "use HTTPS instead of HTTP")
	cert           = flag.String("cert", "cert.pem", "path to the HTTPS certificate pem file")
	key            = flag.String("key", "key.pem", "path to the HTTPS certificate private key pem file")

	watcher *fsnotify.Watcher
	signals chan os.Signal

	hooks hook.Hooks
)

func init() {
	hooks = hook.Hooks{}

	flag.Parse()

	log.SetPrefix("[webhook] ")
	log.SetFlags(log.Ldate | log.Ltime)

	if !*verbose {
		log.SetOutput(ioutil.Discard)
	}

	log.Println("version " + version + " starting")

	// set os signal watcher
	log.Printf("setting up os signal watcher\n")

	signals = make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1)

	go watchForSignals()

	// load and parse hooks
	log.Printf("attempting to load hooks from %s\n", *hooksFilePath)

	err := hooks.LoadFromFile(*hooksFilePath)

	if err != nil {
		log.Printf("couldn't load hooks from file! %+v\n", err)
	} else {
		log.Printf("loaded %d hook(s) from file\n", len(hooks))

		for _, hook := range hooks {
			log.Printf("\t> %s\n", hook.ID)
		}
	}
}

func main() {
	if *hotReload {
		// set up file watcher
		log.Printf("setting up file watcher for %s\n", *hooksFilePath)

		var err error

		watcher, err = fsnotify.NewWatcher()
		if err != nil {
			log.Fatal("error creating file watcher instance", err)
		}

		defer watcher.Close()

		go watchForFileChange()

		err = watcher.Add(*hooksFilePath)
		if err != nil {
			log.Fatal("error adding hooks file to the watcher", err)
		}
	}

	l := negroni.NewLogger()
	l.Logger = log.New(os.Stdout, "[webhook] ", log.Ldate|log.Ltime)

	negroniRecovery := &negroni.Recovery{
		Logger:     l.Logger,
		PrintStack: true,
		StackAll:   false,
		StackSize:  1024 * 8,
	}

	n := negroni.New(negroniRecovery, l)

	router := mux.NewRouter()

	var hooksURL string

	if *hooksURLPrefix == "" {
		hooksURL = "/{id}"
	} else {
		hooksURL = "/" + *hooksURLPrefix + "/{id}"
	}

	router.HandleFunc(hooksURL, hookHandler)

	n.UseHandler(router)

	if *secure {
		log.Printf("starting secure (https) webhook on %s:%d", *ip, *port)
		log.Fatal(http.ListenAndServeTLS(fmt.Sprintf("%s:%d", *ip, *port), *cert, *key, n))
	} else {
		log.Printf("starting insecure (http) webhook on %s:%d", *ip, *port)
		log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *ip, *port), n))
	}

}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	hook := hooks.Match(id)

	if hook != nil {
		log.Printf("%s got matched\n", id)

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("error reading the request body. %+v\n", err)
		}

		// parse headers
		headers := valuesToMap(r.Header)

		// parse query variables
		query := valuesToMap(r.URL.Query())

		// parse body
		var payload map[string]interface{}

		contentType := r.Header.Get("Content-Type")

		if strings.HasPrefix(contentType, "application/json") {
			decoder := json.NewDecoder(strings.NewReader(string(body)))
			decoder.UseNumber()

			err := decoder.Decode(&payload)

			if err != nil {
				log.Printf("error parsing JSON payload %+v\n", err)
			}
		} else if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
			fd, err := url.ParseQuery(string(body))
			if err != nil {
				log.Printf("error parsing form payload %+v\n", err)
			} else {
				payload = valuesToMap(fd)
			}
		}

		hook.ParseJSONParameters(&headers, &query, &payload)

		// handle hook
		go handleHook(hook, &headers, &query, &payload, &body)

		// send the hook defined response message
		fmt.Fprintf(w, hook.ResponseMessage)
	} else {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Hook not found.")
	}
}

func handleHook(hook *hook.Hook, headers, query, payload *map[string]interface{}, body *[]byte) {
	if hook.TriggerRule == nil || hook.TriggerRule != nil && hook.TriggerRule.Evaluate(headers, query, payload, body) {
		log.Printf("%s hook triggered successfully\n", hook.ID)

		cmd := exec.Command(hook.ExecuteCommand)
		cmd.Args = hook.ExtractCommandArguments(headers, query, payload)
		cmd.Dir = hook.CommandWorkingDirectory

		log.Printf("executing %s (%s) with arguments %s using %s as cwd\n", hook.ExecuteCommand, cmd.Path, cmd.Args, cmd.Dir)

		out, err := cmd.Output()

		log.Printf("stdout: %s\n", out)

		if err != nil {
			log.Printf("stderr: %+v\n", err)
		}
		log.Printf("finished handling %s\n", hook.ID)
	} else {
		log.Printf("%s hook did not get triggered\n", hook.ID)
	}
}

func reloadHooks() {
	newHooks := hook.Hooks{}

	// parse and swap
	log.Printf("attempting to reload hooks from %s\n", *hooksFilePath)

	err := newHooks.LoadFromFile(*hooksFilePath)

	if err != nil {
		log.Printf("couldn't load hooks from file! %+v\n", err)
	} else {
		log.Printf("loaded %d hook(s) from file\n", len(hooks))

		for _, hook := range hooks {
			log.Printf("\t> %s\n", hook.ID)
		}

		hooks = newHooks
	}
}

func watchForFileChange() {
	for {
		select {
		case event := <-(*watcher).Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Println("hooks file modified")

				reloadHooks()
			}
		case err := <-(*watcher).Errors:
			log.Println("watcher error:", err)
		}
	}
}

func watchForSignals() {
	log.Println("os signal watcher ready")

	for {
		sig := <-signals
		if sig == syscall.SIGUSR1 {
			log.Println("caught USR1 signal")

			reloadHooks()
		} else {
			log.Printf("caught unhandled signal %+v\n", sig)
		}
	}
}

// valuesToMap converts map[string][]string to a map[string]string object
func valuesToMap(values map[string][]string) map[string]interface{} {
	ret := make(map[string]interface{})

	for key, value := range values {
		if len(value) > 0 {
			ret[key] = value[0]
		}
	}

	return ret
}
