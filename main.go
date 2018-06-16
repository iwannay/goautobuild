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
	noVendor     string
	printHelp    bool
	extMap       = make(map[string]bool, 0)
	watchPath    string
	buildTime    time.Time
	cmd          *exec.Cmd
	lock         sync.Mutex
	vendorDir    string
	tmpVendorDir string
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
	defer func() {
		err := rename(tmpVendorDir, vendorDir)
		if err != nil {
			log.Println("[ERROR] rename err:", err)
		}
		lock.Unlock()
	}()

	if time.Now().Sub(buildTime) > intervalTime {
		var err error
		buildTime = time.Now()
		os.Chdir(watchPath)
		cmdName := "go"

		if noVendor != "" {
			err := rename(vendorDir, tmpVendorDir)
			if err != nil {
				log.Println("[ERROR] rename err:", err)
			}
		}

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
	var err error
	flag.StringVar(&watchPathArg, "d", "./", "监听的目录，默认当前目录.eg:/project")
	flag.StringVar(&watchExtsArg, "e", "", "监听的文件类型，默认监听所有文件类型.eg：'.go','.html','.php'")
	flag.StringVar(&ignoreDirArg, "i", "", "忽略监听的目录")
	flag.BoolVar(&printHelp, "help", false, "显示帮助信息")
	flag.StringVar(&noVendor, "novendor", "", "编译时忽略指定的vendor目录")
	flag.Parse()

	if printHelp {
		flag.Usage()
		return
	}

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

	if noVendor != "" {
		vendorDir, err = filepath.Abs(filepath.Clean(noVendor))
		if err != nil {
			log.Fatalf("[FATAL] %v", err)
		}

		if base := filepath.Base(vendorDir); base != "vendor" {
			log.Fatalf("[FATAL] %s is not vendor dir", base)
		}
		dir := filepath.Dir(vendorDir)
		tmpVendorDir = filepath.Join(dir, "_vendor")
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
		if noVendor != "" {
			err := rename(tmpVendorDir, vendorDir)
			if err != nil {
				log.Println("[ERROR] rename err:", err)
			}
		}
	})

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

				log.Println("[INFO] event type:", event.Op, " filename:", event.Name)

				if (event.Op == fsnotify.Write) && checkFile(event.Name) && filepath.Base(event.Name) != appName {
					log.Println("[INFO] modified file:", event.Name)
					go autobuild()
				}

				if event.Op == fsnotify.Create && checkFile(event.Name) && filepath.Base(event.Name) != appName {
					log.Println("[INFO] add file:", event.Name)
					watcher.Add(event.Name)
					go autobuild()
				}

				if event.Op == fsnotify.Remove || event.Op == fsnotify.Rename && checkFile(event.Name) && filepath.Base(event.Name) != appName {
					log.Println("[INFO] remove file:", event.Name)
					watcher.Remove(event.Name)
					go autobuild()
				}

			case err := <-watcher.Errors:
				log.Println("[ERROR] watcher error:", err)
			}
		}
	}()

	log.Println("[INFO] watch", watchPath, " file ext", extArr)
	err = watcher.Add(watchPath)
	if err != nil {
		log.Fatalf("[FATAL] watcher -> %v", err)
	}
	filepath.Walk(watchPath, func(path string, info os.FileInfo, err error) error {

		for _, v := range ignoreDirArr {
			if v == path {
				return errors.New("ignore " + path)
			}
		}

		if info.IsDir() {

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
