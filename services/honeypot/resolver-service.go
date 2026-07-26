// main.go
//
// ResolvTech IP Resolver (Fictional Company)
// A deliberately vulnerable web server for educational purposes.
// DO NOT deploy this in production or expose it to untrusted networks.



//  set a temporary cache directory, then run your server:
// run it in linux-pivot machine
//export GOCACHE=/tmp/go-cache
//go run main.go


package main

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

func main() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/resolve", resolveHandler)
	log.Println("ResolvTech IP Resolver listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<html>
<head><title>ResolvTech - IP Resolver</title></head>
<body>
<h1>ResolvTech IP Resolver</h1>
<p>Enter an IP address or hostname to resolve:</p>
<form action="/resolve" method="GET">
  <input type="text" name="query" placeholder="e.g. 8.8.8.8 or google.com">
  <input type="submit" value="Resolve">
</form>
</body>
</html>`)
}

func resolveHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Missing 'query' parameter", http.StatusBadRequest)
		return
	}

	// *** VULNERABILITY: Unsanitised input passed directly to a shell command ***
	// An attacker can inject arbitrary commands using shell metacharacters
	// e.g. ?query=127.0.0.1;id
	cmd := exec.Command("sh", "-c", "nslookup "+query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// The error may also contain output, so we still return it.
		output = append(output, []byte("\nError: "+err.Error())...)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "ResolvTech IP Resolver - Result for: %s\n%s\n", query, output)
}
