// The MIT License
//
// Copyright (c) 2017-2018 by the author(s)
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
//
// Author(s):
//   - Andreas Oeldemann <andreas.oeldemann@tum.de>
//
// Description:
//
// Logging facility.

package gofluent10g

import (
	"log"
	"os"
)

// log levels
const (
	LOG_DEBUG int = iota
	LOG_INFO
	LOG_WARN
	LOG_ERR
)

// one logger for each log level
var (
	logDebug       *log.Logger
	logInfo        *log.Logger
	logWarn        *log.Logger
	logError       *log.Logger
	logIndentLevel uint
	logLevel       = LOG_INFO
)

// Log prints out a log message with a specifiable log level.
func Log(level int, msg string, a ...interface{}) {

	if level < logLevel {
		// do not print out log message if criticality is below the one
		// specified by the user
		return
	}

	for i := uint(0); i < logIndentLevel; i++ {
		msg = "... " + msg
	}

	if level == LOG_DEBUG {
		if logDebug == nil {
			logDebug = log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Lmicroseconds)
		}
		logDebug.Printf(msg, a...)
	} else if level == LOG_INFO {
		if logInfo == nil {
			logInfo = log.New(os.Stdout, "INFO: ", log.Ldate|log.Lmicroseconds)
		}
		logInfo.Printf(msg, a...)
	} else if level == LOG_WARN {
		if logWarn == nil {
			logWarn = log.New(os.Stdout, "WARN: ", log.Ldate|log.Lmicroseconds)
		}
		logWarn.Printf(msg, a...)
	} else if level == LOG_ERR {
		if logError == nil {
			logError = log.New(os.Stdout, "ERROR: ", log.Ldate|log.Lmicroseconds)
		}
		logError.Fatalf(msg, a...)
	} else {
		logError.Fatalln("invalid log level")
	}
}

// LogIncrementIndentLevel increments the indentation level of all further log
// messages.
func LogIncrementIndentLevel() {
	logIndentLevel++
}

// LogDecrementIndentLevel decrements the indentation level of all further log
// messages.
func LogDecrementIndentLevel() {
	if logIndentLevel == 0 {
		Log(LOG_ERR, "logIndentLevel reached negative value. Check your code!")
	}
	logIndentLevel--
}

// LogSetLevel sets the minimum criticality of the messages that are actually
// printed. Log messages below the criticality level are ignored.
func LogSetLevel(level int) {
	if level < LOG_DEBUG || level > LOG_ERR {
		Log(LOG_ERR, "invalid log level")
	}
	logLevel = level
}
