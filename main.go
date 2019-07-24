package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
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
	ignoreDirArg string
	ignoreDirArr []string
	watchDirArg  string
	mod          string
	cmdArgs      string
	cmdArgsArr   []string
	printHelp    bool
	extMap       = make(map[string]bool, 0)
	watchPath    string
	buildTime    time.Time
	cmd          *exec.Cmd
	lock         sync.Mutex
)

const (
	intervalTime = 3 * time.Second
	appName      = "binTmp"
)

func checkFile(file string) bool {
	if len(extMap) == 0 {
		return true
	}
	ext := filepath.Ext(file)
	return extMap[ext]

}

func rename(oldpath, newpath string) error {
	finfo, err := os.Stat(oldpath)
	if !os.IsNotExist(err) {
		if finfo.IsDir() {
			_, err := os.Stat(newpath)
			if os.IsNotExist(err) {
				log.Println("[INFO] rename", oldpath, "to", newpath)
				return os.Rename(oldpath, newpath)
			}
			return err

		}
	}
	return nil
}

func listenSignal(fn func()) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill)
	for {
		sign := <-c
		log.Println("get signal:", sign)
		if fn != nil {
			fn()
		}
		log.Fatal("trying to exit gracefully...")

	}
}

func autobuild() {
	lock.Lock()
	defer lock.Unlock()

	if time.Now().Sub(buildTime) > intervalTime {
		var err error
		buildTime = time.Now()
		os.Chdir(watchPath)
		cmdName := "go"

		binName := appName
		if runtime.GOOS == "windows" {
			binName += ".exe"
		}
		args := []string{"build"}

		if mod != "" {
			args = append(args, "-mod", mod)
		}

		args = append(args, "-o", binName)

		cmd := exec.Command(cmdName, args...)
		cmd.Env = append(os.Environ(), "GOGC=off")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		log.Println("[INFO] Start building...")
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
	log.Printf("[INFO] Restarting %s %s ...\n", binName, cmdArgs)
	binName = "./" + binName
	cmd = exec.Command(binName, cmdArgsArr...)
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

func addWatch(root string, watcher *fsnotify.Watcher) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		for _, v := range ignoreDirArr {
			if v == path {
				return errors.New("ignore " + path)
			}
		}

		if err != nil {
			log.Printf("[ERROR] %s", err)
		}

		if info.IsDir() {
			log.Printf("[TRAC] Directory( %s )\n", path)
		}

		if err := watcher.Add(path); err != nil {
			log.Fatalf("[ERROR] Failed to watch directory[ %s ]\n", err)
		}
		return err
	})
}

func removeWatch(root string, watcher *fsnotify.Watcher) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			log.Printf("[TRAC] Directory( %s )\n", path)
		}

		if err := watcher.Remove(path); err != nil {
			log.Fatalf("[ERROR] Failed to remove watch [ %s ]\n", err)
		}
		return err
	})
}

func main() {
	var err error

	flag.StringVar(&watchPathArg, "d", "./", "工作目录，默认当前目录.eg:/project，处在工作目录的文件会被自动监控变化")
	flag.StringVar(&watchExtsArg, "e", "", "监听的文件类型，默认监听所有文件类型.eg：'.go','.html','.php'")
	flag.StringVar(&ignoreDirArg, "i", "", "忽略监听的目录")
	flag.BoolVar(&printHelp, "help", false, "显示帮助信息")
	flag.StringVar(&cmdArgs, "args", "", "自定义命令参数")
	flag.StringVar(&mod, "mod", "", "指定mod使用的vendor")
	flag.StringVar(&watchDirArg, "w", "", "监听的目录")
	flag.Parse()

	if printHelp {
		fmt.Fprintf(flag.CommandLine.Output(), "version: %s\n", "0.5.0")
		flag.Usage()
		return
	}

	cmdArgsArr = strings.Split(cmdArgs, " ")

	if ignoreDirArg != "" {
		ignoreDirArr = strings.Split(ignoreDirArg, ",")
		for k, v := range ignoreDirArr {

			ignoreDirArr[k], err = filepath.Abs(filepath.Clean(v))
			if err != nil {
				log.Fatalf("[FATAL] %v", err)
			}
			log.Println("[INFO] ignore:", ignoreDirArr[k])
		}

	}

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

	go listenSignal(func() {
		kill()
	})

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("[FATAL] watcher -> %s", err)
	}

	defer watcher.Close()

	done := make(chan bool)

	go func() {

		for {
			select {
			case event := <-watcher.Events:

				if !checkFile(event.Name) || filepath.Base(event.Name) == appName {
					continue
				}

				if event.Op == fsnotify.Write {
					log.Println("[INFO] Write: ", event.Name)
					go autobuild()
				}

				if event.Op == fsnotify.Create {
					log.Println("[INFO] Create: ", event.Name)
					addWatch(event.Name, watcher)
					go autobuild()
				}

				if event.Op == fsnotify.Remove || event.Op == fsnotify.Rename {
					log.Println("[INFO] Remove: ", event.Name)
					removeWatch(event.Name, watcher)
					go autobuild()
				}

			case err := <-watcher.Errors:
				log.Println("[ERROR] watcher error:", err)
			}
		}
	}()

	var watchDir []string
	watchDir = append(watchDir, watchPath)
	if watchDirArg != "" {
		watchDir = append(watchDir, strings.Split(watchDirArg, ",")...)
	}
	for _, v := range watchDir {
		log.Println("[INFO] watch", v, ",file ext", extArr)
		err = watcher.Add(v)
		if err != nil {
			log.Fatalf("[FATAL] watcher -> %v", err)
		}
		addWatch(v, watcher)
	}

	autobuild()
	<-done
}
