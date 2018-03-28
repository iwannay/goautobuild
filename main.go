package main

import (
	"flag"
	"log"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

var watchPathArg = flag.String("-p", "./", "监听的目录，默认当前目录.eg:/project")
var watchExtsArg = flag.String("-e", "", "监听的文件类型，默认监听所有文件类型.eg：.go,.html.php ")
var extMap = make(map[string]bool, 0)
var watchPath string

func checkFile(file string) bool {
	if len(extMap) == 0 {
		return true
	}
	ext := filepath.Ext(file)
	_, ok := extMap[ext]
	return ok
}

func autobuild() {

}

func main() {

	flag.Parse()

	watchPath = filepath.Clean(*watchPathArg)
	extArr := strings.Split(*watchExtsArg, ",")

	if len(extArr) > 0 {

		for _, v := range extArr {
			if strings.TrimSpace(v) != "" {
				extMap[v] = true
			}
		}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	defer watcher.Close()

	done := make(chan bool)

	go func() {

		for {
			select {
			case event := <-watcher.Events:

				if (event.Op&fsnotify.Write == fsnotify.Write) && checkFile(event.Name) {
					log.Println("modified file:", event.Name)
				}

			case err := <-watcher.Errors:
				log.Println("errors:", err)
			}
		}
	}()

	log.Println("watch", watchPath, "filter file ext", extArr)
	err = watcher.Add(watchPath)
	if err != nil {
		log.Fatal(err)
	}
	<-done
}
