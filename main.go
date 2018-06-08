package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	watchPathArg string
	watchExtsArg string
	ignoreDir    string
	printHelp    string
	extMap       = make(map[string]bool, 0)
	watchPath    string
	buildTime    time.Time
	cmd          *exec.Cmd
	lock         sync.Mutex
)

const (
	intervalTime = 2 * time.Second
	appName      = "binTmp"
)

func checkFile(file string) bool {
	if len(extMap) == 0 {
		return true
	}
	ext := filepath.Ext(file)
	return extMap[ext]

}

func autobuild() {
	lock.Lock()
	defer lock.Unlock()

	if time.Now().Sub(buildTime) > intervalTime {
		buildTime = time.Now()
		log.Println("[INFO] Start building...")
		os.Chdir(watchPath)
		cmdName := "go"

		var err error
		binName := appName
		if runtime.GOOS == "windows" {
			binName += ".exe"
		}
		args := []string{"build"}
		args = append(args, "-o", binName)

		cmd := exec.Command(cmdName, args...)
		cmd.Env = append(os.Environ(), "GOGC=off")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()

		if err != nil {
			log.Println("[ERROR]================Build failed=================")
			return
		}

		log.Println("[SUCCESS] Build success")
		restart(binName)
	}
}

func restart(binName string) {
	log.Println("[INFO] Kill running process")
	kill()
	go start(binName)
}

func kill() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("[ERROR] Kill failed recover -> ", err)
		}
	}()
	log.Println("[INFO] Killing process")

	if cmd != nil && cmd.Process != nil {
		err := cmd.Process.Kill()
		if err != nil {
			fmt.Println("[ERROR] Kill process -> ", err)

		}
		log.Println("[SUCCESS] Kill process success")

	}
	log.Println("[info] this process is nil")

}

func start(binName string) {
	log.Printf("[INFO] Restarting %s ...\n", binName)

	binName = "./" + binName
	cmd = exec.Command(binName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "")
	go cmd.Run()
	log.Printf("[INFO] %s is running...\n", binName)
}

func getCurrentDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}
	return strings.Replace(dir, "\\", "/", -1)
}

func main() {
	flag.StringVar(&watchPathArg, "d", "./", "监听的目录，默认当前目录.eg:/project")
	flag.StringVar(&watchExtsArg, "e", "", "监听的文件类型，默认监听所有文件类型.eg：'.go','.html','.php'")
	flag.StringVar(&ignoreDir, "i", "", "忽略监听的目录")
	flag.StringVar(&printHelp, "-help", "", "显示帮助信息")

	flag.Parse()
	var err error
	watchPath, err = filepath.Abs(filepath.Clean(watchPathArg))
	if err != nil {
		log.Fatalf("[FATAL] %v", err)
	}
	extArr := strings.Split(watchExtsArg, ",")

	if len(extArr) > 0 {

		for _, v := range extArr {
			if strings.TrimSpace(v) != "" {
				extMap[v] = true
			}
		}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("[FATAL] watcher ->", err)
	}

	defer watcher.Close()

	done := make(chan bool)

	go func() {

		for {
			select {
			case event := <-watcher.Events:

				if (event.Op == fsnotify.Write) && checkFile(event.Name) && filepath.Base(event.Name) != appName {
					log.Println("[INFO] modified file:", event.Name)
					go autobuild()
				}

				if event.Op == fsnotify.Create && checkFile(event.Name) && filepath.Base(event.Name) != appName {
					log.Println("[INFO] add file:", event.Name)
					watcher.Add(event.Name)
					go autobuild()
				}

				if event.Op == fsnotify.Remove && checkFile(event.Name) && filepath.Base(event.Name) != appName {
					log.Println("[INFO] remote file:", event.Name)
					watcher.Remove(event.Name)
					go autobuild()
				}

			case err := <-watcher.Errors:
				log.Println("[ERROR] watcher -> %v", err)
			}
		}
	}()

	log.Println("[INFO] watch", watchPath, " file ext", extArr)
	err = watcher.Add(watchPath)

	ignoreDir = filepath.Join(getCurrentDirectory(), ignoreDir)
	if err != nil {
		log.Fatalf("[FATAL] watcher -> %v", err)
	}
	filepath.Walk(watchPath, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() && path != ignoreDir {
			log.Printf("[TRAC] Directory( %s )\n", path)
			err := watcher.Add(path)
			if err != nil {
				log.Fatalf("[ERROR] Fail to watch directory[ %s ]\n", err)
			}
		}
		return err
	})
	autobuild()
	<-done
}
