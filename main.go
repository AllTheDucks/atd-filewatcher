package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/go-fsnotify/fsnotify"
)

var watcher fsnotify.Watcher

var buildCmd, appCmd *exec.Cmd = nil, nil

func main() {
	var buildCmdArg, runCmdArg, filePattern string
	flag.StringVar(&buildCmdArg, "build-cmd", "", "The command to run to process changed files e.g. \"go build\"")
	flag.StringVar(&runCmdArg, "run-cmd", "", "The command to run after files are processed e.g. \"./my-app\"")
	flag.StringVar(&filePattern, "file-pattern", "", "Files to monitor for changes e.g. \"*.go\"")

	flag.Parse()

	if buildCmdArg == "" || runCmdArg == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	tokens := strings.Split(buildCmdArg, " ")
	if len(tokens) < 1 {
		fmt.Errorf("not enough arguments in build command")
		os.Exit(1)
	}
	buildCmdName := tokens[0]
	buildCmdArgs := []string{}
	if len(tokens) > 1 {
		buildCmdArgs = tokens[1:]
	}

	tokens = strings.Split(runCmdArg, " ")
	if len(tokens) < 1 {
		fmt.Errorf("not enough arguments in build command")
		os.Exit(1)
	}
	appCmdName := tokens[0]
	appCmdArgs := []string{}
	if len(tokens) > 1 {
		appCmdArgs = tokens[1:]
	}

	changeMsgs := make(chan string, 5)
	done := make(chan bool)
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	err = watcher.Add(".")
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			select {
			case ev := <-watcher.Events:

				if matches, _ := filepath.Match(filePattern, filepath.Base(ev.Name)); ev.Op&(fsnotify.Create|fsnotify.Write) != 0 &&
					matches {
					log.Printf("file: %v had event: %v\n", ev.Name, ev)
					select {
					case changeMsgs <- "file modified":
						fmt.Println("sent message")
					default:
						fmt.Println("no message sent")
					}
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		//		fmt.Printf("File path: %v\n", path)
		if !(strings.HasPrefix(filepath.Base(path), ".") && path != ".") {
			//			fmt.Printf("File Base: %v\n", filepath.Base(path))
			if info.IsDir() {
				fmt.Printf("Watching file: ./%v\n", path)
				addErr := watcher.Add(path)
				if addErr != nil {
					log.Printf("Error while adding: %v\n", addErr)
				}
			}
		} else {
			fmt.Println("Skipping directory ", path)
			if info.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})

	fmt.Println(len(os.Args), os.Args)
	go func() {

		for {
			//TODO need to empty the channel each time
			log.Printf("Listening for change messages.")
			<-changeMsgs
			killApp()

			log.Println("Building App.")
			buildCmd = executeCmd(buildCmdName, buildCmdArgs...)
			log.Println("Built.")

			if err := buildCmd.Wait(); err != nil {
				log.Println(err)
				continue
			} else {
				log.Println("running App...")
				appCmd = executeCmd(appCmdName, appCmdArgs...)
				log.Println("App running.")

			}
		}
	}()

	go func() {
		sig := <-sigs
		fmt.Println(sig)
		killApp()
		done <- true
	}()

	changeMsgs <- "start process"
	<-done

}

func executeCmd(name string, args ...string) (cmd *exec.Cmd) {
	cmd = exec.Command(name, args...)

	fmt.Printf("Path: %v\n", cmd.Path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}
	msgs := make(chan string)

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println(scanner.Text()) // Println will add back the final '\n'
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
		msgs <- "stdout finished"
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Println(scanner.Text()) // Println will add back the final '\n'
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
		msgs <- "stderr finished"
	}()

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	return

}

func killApp() {
	if appCmd != nil {
		log.Println("Attempting to kill app...")
		err := appCmd.Process.Kill()
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Waiting for app to exit...")
		appCmd.Wait()
		log.Println("App exited.")
	}
}
