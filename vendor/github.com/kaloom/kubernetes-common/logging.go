/*
Copyright 2018-2019 Kaloom Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"log"
	"os"
	"strconv"
)

const (
	loggingLevelEnvironmentVar = "_CNI_LOGGING_LEVEL"
	loggingFileEnvironmentVar  = "_CNI_LOGGING_FILE"

	// LoggingLevelNone for no logging
	LoggingLevelNone loggingLevelType = iota
	// LoggingLevelError for error level logging
	LoggingLevelError
	// LoggingLevelInfo for info level logging
	LoggingLevelInfo
	// LoggingLevelDebug for debug level logging
	LoggingLevelDebug
)

var (
	loggingEnabled bool
	loggingLevel   loggingLevelType
)

type loggingLevelType int

// IsLoggingEnabled return true if loggingLevelEnvironmentVar is set
func IsLoggingEnabled() bool {
	if val, present := os.LookupEnv(loggingLevelEnvironmentVar); present {
		if v, err := strconv.Atoi(val); err == nil {
			loggingLevel = getLoggingLevel(v)
			if loggingLevel > LoggingLevelNone {
				return true
			}
		}
	}
	return false
}

// GetLoggingFile return the environment variable value of the loggingFileEnvironmentVar
func GetLoggingFile() string {
	if val, present := os.LookupEnv(loggingFileEnvironmentVar); present {
		return val
	}
	return ""
}

func getLoggingLevel(l int) loggingLevelType {
	var level loggingLevelType
	switch l {
	case 1:
		level = LoggingLevelError
	case 2:
		level = LoggingLevelInfo
	case 3:
		level = LoggingLevelDebug
	default:
		if l > int(LoggingLevelDebug) {
			level = LoggingLevelDebug
		}
	}
	return level
}

// LogInfo info level logging function
func LogInfo(format string, args ...interface{}) {
	if loggingLevel >= LoggingLevelInfo {
		log.Printf("[INFO] "+format, args...)
	}
}

// LogError error level logging function
func LogError(format string, args ...interface{}) {
	if loggingLevel >= LoggingLevelError {
		log.Printf("[ERR] "+format, args...)
	}
}

// LogDebug debug level logging function
func LogDebug(format string, args ...interface{}) {
	if loggingLevel >= LoggingLevelDebug {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// OpenLogFile initializes logging
// - if loggingLevelEnvironmentVar is not defined or not enabled the log functions would be a nop
// - if loggingFileEnvironmentVar is not defined or empty log to stderr
func OpenLogFile() *os.File {
	loggingEnabled = IsLoggingEnabled()
	if loggingEnabled {
		logFile := GetLoggingFile()
		if logFile == "" {
			return os.Stderr
		}
		f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			loggingEnabled = false
		} else {
			log.SetOutput(f)
			return f
		}
	}
	return nil
}

// CloseLogFile finalizes logging
func CloseLogFile(f *os.File) {
	if f != nil {
		f.Close()
	}
}
