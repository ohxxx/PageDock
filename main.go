// [INPUT]: Process environment and MyGo package metadata.
// [OUTPUT]: Start the PageDock desktop app on the main thread.
// [POS]: The single executable entry point; internal owns the desktop lifecycle.
// [PROTOCOL]: Update the root AGENTS.md; update mygo.json if the entry path changes.
package main

import pagedock "github.com/ohxxx/PageDock/internal"

func main() {
	pagedock.Run()
}
