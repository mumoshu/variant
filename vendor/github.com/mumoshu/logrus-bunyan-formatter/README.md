# Logrus Bunyan Log Formatter

[Logrus](https://github.com/Sirupsen/logrus) formatter mainly based on original `logrus.JSONFormatter` but with slightly
modified output and support for [bunyan](https://github.com/trentm/node-bunyan).

## Example

You can go to the `examples/` directory and run this example on your own.

```
$ go run basic.go
{"animal":"walrus","hostname":"mumoshu","level":20,"msg":"Started observing beach","name":"myapp","number":8,"pid":77989,"prefix":"main","time":"2016-09-08T17:29:54+09:00","v":0}
{"animal":"walrus","hostname":"mumoshu","level":20,"msg":"[main] A group of walrus emerges from the ocean","name":"myapp","pid":77989,"size":10,"time":"2016-09-08T17:29:54+09:00","v":0}
{"hostname":"mumoshu","level":40,"msg":"[main] The group's number increased tremendously!","name":"myapp","number":122,"omg":true,"pid":77989,"time":"2016-09-08T17:29:54+09:00","v":0}
{"hostname":"mumoshu","level":30,"msg":"Temperature changes","name":"myapp","pid":77989,"prefix":"sensor","temperature":-4,"time":"2016-09-08T17:29:54+09:00","v":0}
{"animal":"orca","hostname":"mumoshu","level":"panic","msg":"It's over 9000!","name":"myapp","pid":77989,"prefix":"sensor","size":9009,"time":"2016-09-08T17:29:54+09:00","v":0}
{"hostname":"mumoshu","level":60,"msg":"[main] The ice breaks!","name":"myapp","number":100,"omg":true,"pid":77989,"time":"2016-09-08T17:29:54+09:00","v":0}
exit status 1
```

Now, as messages are compatible with bunyan, you can pipe those to `bunyan` cli like:

```
$ go run basic.go | bunyan -o short
exit status 1
08:29:59.000Z DEBUG myapp: Started observing beach (animal=walrus, number=8, prefix=main)
08:29:59.000Z DEBUG myapp: [main] A group of walrus emerges from the ocean (animal=walrus, size=10)
08:29:59.000Z  WARN myapp: [main] The group's number increased tremendously! (number=122, omg=true)
08:29:59.000Z  INFO myapp: Temperature changes (prefix=sensor, temperature=-4)
08:29:59.000Z LVLpanic myapp: It's over 9000! (animal=orca, prefix=sensor, size=9009)
08:29:59.000Z FATAL myapp: [main] The ice breaks! (number=100, omg=true)
```

## Installation
To install formatter, use `go get`:

```sh
$ go get github.com/mumoshu/logrus-bunyan-formatter
```

## Usage
Here is how it should be used:

```go
package main

import (
	"github.com/Sirupsen/logrus"
	bunyan "github.com/mumoshu/logrus-bunyan-formatter"
)

var log = logrus.New()

func init() {
	log.Formatter = new(bunyan.Formatter)
	log.Level = logrus.DebugLevel
}

func main() {
	log.WithFields(logrus.Fields{
		"prefix": "main",
		"animal": "walrus",
		"number": 8,
	}).Debug("Started observing beach")

	log.WithFields(logrus.Fields{
		"prefix":      "sensor",
		"temperature": -4,
	}).Info("Temperature changes")
}
```

## API
`bunyan.Formatter` exposes the following fields:

* `TimestampFormat string` — timestamp format to use for display when a full timestamp is printed.
* `Name string` — applicatio name to be included in each log message.

# License
MIT
