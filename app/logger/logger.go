package logger

import (
	"log"
	"os"
)

var (
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
	DebugLogger *log.Logger
)

func Init() {
	InfoLogger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	DebugLogger = log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func Info(v ...interface{}) {
	InfoLogger.Println(v...)
}

func Error(v ...interface{}) {
	ErrorLogger.Println(v...)
}

func Debug(v ...interface{}) {
	DebugLogger.Println(v...)
}

func Infof(format string, v ...interface{}) {
	InfoLogger.Printf(format, v...)
}

func Errorf(format string, v ...interface{}) {
	ErrorLogger.Printf(format, v...)
}

func Debugf(format string, v ...interface{}) {
	DebugLogger.Printf(format, v...)
}
