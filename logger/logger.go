package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	DebugLevel  = Level(1)
	InfoLevel   = Level(2)
	WarnLevel   = Level(3)
	ErrorLevel  = Level(4)
	FileMaxLine = 65536
)

type Level int32

func (level Level) String() string {
	switch level {
	case DebugLevel:
		return "[debug]"
	case InfoLevel:
		return "[info]"
	case WarnLevel:
		return "[warn]"
	case ErrorLevel:
		return "[error]"
	default:
		return "[none]"
	}
}

var (
	nodeName string
	appName  string
	podName  string
	podInfo  string
	logDir   string

	mutex         sync.Mutex
	stdWrite      io.Writer
	logFile       *os.File
	outputBuf     []byte
	lineCount     int32
	createFileDay int32
	deleteFileDay int32
)

func getSystemEnvAndCmdArg() map[string]string {
	m := make(map[string]string)
	for _, env := range os.Environ() {
		ss := strings.SplitN(env, "=", 2)
		k := ss[0]
		if len(k) > 0 && len(ss) > 1 {
			v := ss[1]
			m[k] = v
		}
	}

	for i := 0; i < len(os.Args); i++ {
		s := os.Args[i]
		if strings.HasPrefix(s, "--") {
			ss := strings.SplitN(strings.TrimPrefix(s, "--"), "=", 2)
			k, v := ss[0], ""
			if len(ss) > 1 {
				v = ss[1]
			}
			m[k] = v
			continue
		}
	}
	return m
}

func init() {
	m := getSystemEnvAndCmdArg()
	key := "NODE_NAME"
	nodeName = ""
	if val, ok := m[key]; ok {
		nodeName = val
		podInfo = podInfo + fmt.Sprintf("[%s]", nodeName)
	}

	key = "POD_NAME"
	podName = ""
	if val, ok := m[key]; ok {
		podName = val
		podInfo = podInfo + fmt.Sprintf("[%s]", podName)
	}

	key = "APP_NAME"
	appName = ""
	if val, ok := m[key]; ok {
		appName = val
		podInfo = podInfo + fmt.Sprintf("[%s] ", appName)
	}

	stdWrite = os.Stdout
	key = "LOG_DIR"
	logDir = "./logs/"
	if val, ok := m[key]; ok {
		logDir = val
	}
	tryNewFile(true)
	go mainloop()
}

// logs/appname/podname.time.log
func tryNewFile(force bool) {
	// 日期不一样或者行数达到上限
	if lineCount > FileMaxLine || createFileDay != int32(time.Now().YearDay()) || force {
		// builf file path
		timeStr := time.Now().Format("20060102150405")
		fileDir := fmt.Sprintf("%s%s", logDir, appName)
		filePath := fmt.Sprintf("%s/%s.log", fileDir, timeStr)
		if podName != "" {
			filePath = fmt.Sprintf("%s/%s.%s.log", fileDir, podName, timeStr)
		}

		//try create dir
		_, err := os.Stat(fileDir)
		if err != nil {
			if os.IsNotExist(err) {
				err = os.MkdirAll(fileDir, os.ModePerm)
				if err != nil {
					panic(fmt.Sprintf("create forder fail fileDir=:%s err:%s", fileDir, err))
				}
			}
		}
		// new file
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			panic(fmt.Sprintf("open log file failed, err:%s", err))
		}
		lineCount = 0
		createFileDay = int32(time.Now().YearDay())
		if logFile != nil {
			logFile.Close()
		}
		logFile = file
	}
}

func mainloop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			tryDelteFile(&now)
		}
	}
}

func tryDelteFile(date *time.Time) {
	// 每日凌晨删除
	if deleteFileDay != int32(date.YearDay()) {
		deleteFileDay = int32(date.YearDay())
		fileDir := fmt.Sprintf("%s%s", logDir, appName)
		fileInfoList, err := os.ReadDir(fileDir)
		if err != nil {
			return
		}

		for i := range fileInfoList {
			fileName := fileInfoList[i].Name()
			filePath := fileDir + fileName
			if needDelete(date, filePath) {
				os.Remove(filePath)
				fmt.Printf("delete file%s \n", filePath)
			}
		}
	}
}

func needDelete(date *time.Time, filePath string) bool {
	f, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("needDeletel open file:[%s] fail err:%v \n", filePath, err)
		return false
	}

	fi, err := f.Stat()
	f.Close()
	if err != nil {
		fmt.Printf("needDelete Stat file:[%s] fail err:%v \n", filePath, err)
		return false
	}

	if date.Unix()-fi.ModTime().Unix() > 3600*24*7 {
		return true
	}

	return false
}

func formatAndWrite(l Level, format string, v ...any) {
	now := time.Now()
	mutex.Lock()
	defer mutex.Unlock()

	outputBuf = outputBuf[:0]
	formatHeader(&outputBuf, l, now)
	s := fmt.Sprintf(format, v...)
	outputBuf = append(outputBuf, s...)
	outputBuf = append(outputBuf, '\n')
	stdWrite.Write(outputBuf)
	logFile.Write(outputBuf)
	lineCount++
	tryNewFile(false)
}

// [level][time][NODE_NAME][POD_NAME][APP_NAME] msg
func formatHeader(buf *[]byte, l Level, t time.Time) {
	*buf = append(*buf, l.String()...)
	timeStr := t.Format("[2006-01-02 15:04:05.000000]")
	*buf = append(*buf, timeStr...)
	*buf = append(*buf, podInfo...)
}

func Infof(format string, v ...any) {
	formatAndWrite(InfoLevel, format, v...)
}

func Warnf(format string, v ...any) {
	formatAndWrite(WarnLevel, format, v...)
}

func Errorf(format string, v ...any) {
	formatAndWrite(ErrorLevel, format, v...)
}

func Debugf(format string, v ...any) {
	formatAndWrite(DebugLevel, format, v...)
}

func Info(v ...any) {
	Infof(fmt.Sprint(v...))
}

func Warn(v ...any) {
	Warnf(fmt.Sprint(v...))
}

func Error(v ...any) {
	Errorf(fmt.Sprint(v...))
}

func Debug(v ...any) {
	Debugf(fmt.Sprint(v...))
}

func JsonInfo(format string, v any) {
	bb, e := json.Marshal(v)
	if e != nil {
		Errorf("e:%s", e)
		return
	}

	Debugf(format, string(bb))
}
