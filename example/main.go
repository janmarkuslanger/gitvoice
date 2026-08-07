// Command example shows the intended integration: a repository that manages
// its invoices with gitvoice needs only this file plus a go.mod.
package main

import (
	"log"

	"github.com/janmarkuslanger/gitvoice"
)

func main() {
	err := gitvoice.Run(gitvoice.Config{
		DataDir: "data",           // JSON files live here — commit them with git
		Addr:    "127.0.0.1:8080", // local only; put auth in front before exposing it
	})
	if err != nil {
		log.Fatal(err)
	}
}
