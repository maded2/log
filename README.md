# Opinionated Logging for Golang

TWO logging levels only

1. ForOps - logging for Operation Staff to monitor
2. ForDev - logging for Developers


> Features
* Log to console and/or file
* File logging rotate of daily basis
* Logging configuration is stored on the filesystem in a JSON format
* Configuration file is monitored for changes every 10 secs
* Dynamic suppression of Dev logging defined in the logging configuration
