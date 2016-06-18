package log

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"
)

func Config(configFile string, filenameOverride string) {
	defaultLogger.configFile = configFile
	defaultLogger.fileSink = make(chan *logEntry, 100)
	defaultLogger.loadConfig()
	if len(filenameOverride) > 0 {
		defaultLogger.config.ForFile = filenameOverride
	}
}

func ForDev(context string, msg string, args ...interface{}) {
	if dc, found := defaultLogger.config.DevContexts[context]; (!found || dc == false) && defaultLogger.config.LogAllDev == false {
		return
	}
	source := ""
	if pc, _, lineno, ok := runtime.Caller(1); ok {
		source = fmt.Sprintf("%s:%d", runtime.FuncForPC(pc).Name(), lineno)
	}
	if defaultLogger.config.ForConsole {
		entry := defaultLogger.pool.Get().(*logEntry)
		entry.dev = true
		entry.context = context
		entry.msg = msg
		entry.params = args
		entry.source = source
		defaultLogger.consoleSink <- entry
	}

	if defaultLogger.fileSink != nil {
		entry := defaultLogger.pool.Get().(*logEntry)
		entry.dev = true
		entry.context = context
		entry.msg = msg
		entry.params = args
		entry.source = source
		defaultLogger.fileSink <- entry
	}
}

func ForOps(msg string, args ...interface{}) {
	if defaultLogger.config.ForConsole {
		entry := defaultLogger.pool.Get().(*logEntry)
		entry.dev = false
		entry.msg = msg
		entry.params = args
		defaultLogger.consoleSink <- entry
	}

	if defaultLogger.fileSink != nil {
		entry := defaultLogger.pool.Get().(*logEntry)
		entry.dev = false
		entry.msg = msg
		entry.params = args
		defaultLogger.fileSink <- entry
	}
}

type LogConfig struct {
	ForConsole  bool            `json:"console"`
	ForFile     string          `json:"filename"`
	LogAllDev   bool            `json:"log-all-dev"`
	DevContexts map[string]bool `json:"dev-contexts"`
}

var defaultLogger *Logger

type Logger struct {
	configFile      string
	lastModTime     time.Time
	consoleSink     chan *logEntry
	fileSink        chan *logEntry
	fileHandle      *os.File
	pool            sync.Pool
	config          *LogConfig
	nextRollLogFile time.Time
}

func (logger *Logger) loop() {
	configCheckTicker := time.NewTicker(time.Second * 10)
	fileLogSyncTicker := time.NewTicker(time.Second)
	for {
		select {
		case <-configCheckTicker.C:
			logger.checkConfigFile()
		case <-fileLogSyncTicker.C:
			if logger.fileHandle != nil {
				logger.fileHandle.Sync()
			}
		case entry := <-logger.consoleSink:
			logger.logToConsole(entry)
		case entry := <-logger.fileSink:
			logger.logToFile(entry)
		}
	}
}

func (logger *Logger) checkConfigFile() {
	if len(logger.configFile) == 0 {
		return
	}
	if info, err := os.Stat(logger.configFile); err == nil && info.ModTime().After(logger.lastModTime) {
		logger.loadConfig()
	}
}

func (logger *Logger) loadConfig() {
	if len(logger.configFile) == 0 {
		return
	}
	fmt.Fprintf(os.Stdout, "\033[0;42mLoading logging configuration [%s]\033[0m\n", logger.configFile)
	if file, err := os.Open(logger.configFile); err != nil {
		fmt.Fprintf(os.Stdout, "\033[0;42mFailed to open log config file: %s\033[0m\n", err)
	} else {
		decoder := json.NewDecoder(file)
		config := LogConfig{}
		err = decoder.Decode(&config)
		if err != nil {
			fmt.Fprintf(os.Stdout, "\033[0;42mFailed to process log config file: %s\033[0m\n", err)
		} else {
			defaultLogger.config = &config
		}
		if info, err := file.Stat(); err == nil {
			logger.lastModTime = info.ModTime()
		}
	}
}

func (logger *Logger) logToConsole(entry *logEntry) {
	msg := entry.msg
	if len(entry.params) > 0 {
		msg = fmt.Sprintf(entry.msg, entry.params...)
	}
	if entry.dev {
		fmt.Fprint(os.Stdout, "DEV \033[0;35m", time.Now().Local().Format("[02/01/06 15:04:05.000] ["), entry.context, "]\033[0m [", entry.source, "] ", msg, "\n")
	} else {
		fmt.Fprint(os.Stdout, "OPS \033[0;34m", time.Now().Local().Format("[02/02/06 15:04:05.000]"), "\033[0m ", msg, "\n")
	}
	logger.pool.Put(entry)
}

func (logger *Logger) logToFile(entry *logEntry) {
	logger.checkLog()
	if logger.fileHandle != nil {
		msg := entry.msg
		if len(entry.params) > 0 {
			msg = fmt.Sprintf(entry.msg, entry.params...)
		}
		if entry.dev {
			fmt.Fprint(logger.fileHandle, time.Now().Local().Format("DEV [02/01/06 15:04:05.000] ["), entry.context, "] [", entry.source, "] ", msg, "\n")
		} else {
			if _, err := fmt.Fprint(logger.fileHandle, time.Now().Local().Format("OPS [02/01/06 15:04:05.000] "), msg, "\n"); err != nil {
				fmt.Fprintln(os.Stdout, "Failed to write to file: ", err)
			}
		}
	} else {
		fmt.Fprintln(os.Stdout, "not logging to file")
	}
	logger.pool.Put(entry)
}

func (logger *Logger) checkLog() {
	if logger.fileHandle != nil && time.Now().Local().After(logger.nextRollLogFile) {
		logger.fileHandle.Close()
		logger.fileHandle = nil
	}
	if logger.fileHandle == nil {
		today := time.Now().Local()
		today= time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
		name := fmt.Sprint(logger.config.ForFile, today.Format("-20060102.log"))
		fmt.Fprintf(os.Stdout, "\033[0;42mCreating log file [%s] %s\033[0m\n", name, today)
		var err error
		logger.fileHandle, err = os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0777)
		if err != nil {
			fmt.Fprintf(os.Stdout, "\033[0;42mFailed to create log file [%s]: %s\033[0m\n", name, err)
			return
		}
		logger.nextRollLogFile = today.Add(time.Hour * 24)
	}
}

func init() {
	fmt.Fprintln(os.Stdout, "\033[0;42mInit Default Logger\033[0m")
	defaultLogger = &Logger{
		consoleSink: make(chan *logEntry, 100),
		pool: sync.Pool{
			New: func() interface{} {
				return &logEntry{}
			},
		},
		config: &LogConfig{
			LogAllDev: true,
			DevContexts: map[string]bool{
				"DB": true,
			},
		},
	}
	go defaultLogger.loop()
}

type logEntry struct {
	dev     bool
	context string
	source  string
	msg     string
	params  []interface{}
}
