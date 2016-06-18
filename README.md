# Opinionated Logging for Golang

TWO logging level
. ForOps - logging for Operation Staff to monitor
. ForDev - logging for Developers

Log to console and/or file
File logging rotate of daily basis
Logging configuration is stored on the filesystem and it is monitored for changes every 10 secs
Dynamic suppression of Dev logging defined in the logging configuration
