// Craig Hesling
// May 25, 2018
//
// This is a simple OpenChirp service that outputs the windoed running average
// of a data stream.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/openchirp/framework"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	version string = "1.0"
)

const (
	configKeyInputTopics  = "InputTopics"
	configKeyOutputTopics = "OutputTopics"
	configWindowsSizes    = "WindowSizes"
)

const (
	defaultWindowSize        = 2
	defaultOutputTopicSuffix = "_avg"
)

const (
	// Set this value to true to have the service publish a service status of
	// "Running" each time it receives a device update event
	runningStatus = true
)

func commaList(str string) []string {
	nospacestr := strings.Replace(str, " ", "", -1)
	elements := strings.Split(nospacestr, ",")
	if len(elements) == 1 && len(elements[0]) == 0 {
		return []string{}
	}
	return elements
}

// Device holds the device specific last values and target topics for the difference.
type Device struct {
	outtopics  []string
	lastvalues [][]float64
	nextindex  []int
}

// NewDevice is called by the framework when a new device has been linked.
func NewDevice() framework.Device {
	d := new(Device)
	return framework.Device(d)
}

func (d *Device) addLastValue(topicIndex int, value float64) {
	nextIndex := d.nextindex[topicIndex]
	d.lastvalues[topicIndex][nextIndex] = value
	d.nextindex[topicIndex] = (nextIndex + 1) % len(d.lastvalues[topicIndex])
}

func (d *Device) calculateAverage(topicIndex int) float64 {
	var sum float64
	for _, val := range d.lastvalues[topicIndex] {
		sum += val
	}
	return sum / float64(len(d.lastvalues[topicIndex]))
}

// ProcessLink is called once, during the initial setup of a
// device, and is provided the service config for the linking device.
func (d *Device) ProcessLink(ctrl *framework.DeviceControl) string {
	logitem := log.WithField("deviceid", ctrl.Id())
	logitem.Debug("Linking with config:", ctrl.Config())

	// Allows space in comma seperated list
	inputTopics := commaList(ctrl.Config()[configKeyInputTopics])
	outputTopics := commaList(ctrl.Config()[configKeyOutputTopics])
	windowSizes := commaList(ctrl.Config()[configWindowsSizes])

	d.outtopics = make([]string, len(inputTopics))
	d.lastvalues = make([][]float64, len(inputTopics))
	d.nextindex = make([]int, len(inputTopics))

	for i, intopic := range inputTopics {
		var outtopic string
		if i < len(outputTopics) {
			outtopic = outputTopics[i]
		} else {
			// if no putput topic specified, simply append a _diff to the topic
			outtopic = intopic + defaultOutputTopicSuffix
		}
		d.outtopics[i] = outtopic

		var winsize int = defaultWindowSize
		if i < len(windowSizes) {
			val, err := strconv.ParseInt(windowSizes[i], 10, 32)
			if err != nil {
				logitem.Warnf("Failed to parse WindowSize. Given \"%s\".", windowSizes[i])
				return "Failed to parse WindowSize"
			}
			if val > 0 {
				winsize = int(val)
			}
		}
		d.lastvalues[i] = make([]float64, winsize)

		ctrl.Subscribe(framework.TransducerPrefix+"/"+intopic, i)
	}

	logitem.Debug("Finished Linking")

	// This message is sent to the service status for the linking device
	return "Success"
}

// ProcessUnlink is called once, when the service has been unlinked from
// the device.
func (d *Device) ProcessUnlink(ctrl *framework.DeviceControl) {
	logitem := log.WithField("deviceid", ctrl.Id())
	logitem.Debug("Unlinked:")
}

// ProcessConfigChange is ignored in this case.
func (d *Device) ProcessConfigChange(ctrl *framework.DeviceControl, cchanges, coriginal map[string]string) (string, bool) {
	logitem := log.WithField("deviceid", ctrl.Id())

	logitem.Debug("Ignoring Config Change:", cchanges)
	return "", false
}

// ProcessMessage is called upon receiving a pubsub message destined for
// this device.
func (d *Device) ProcessMessage(ctrl *framework.DeviceControl, msg framework.Message) {
	logitem := log.WithField("deviceid", ctrl.Id())
	logitem.Debugf("Processing avg for topic %s", msg.Topic())

	index := msg.Key().(int)
	value, err := strconv.ParseFloat(string(msg.Payload()), 64)
	if err != nil {
		logitem.Warnf("Failed to convert message (\"%v\") to float64", string(msg.Payload()))
		return
	}

	d.addLastValue(index, value)
	avg := d.calculateAverage(index)

	logitem.Debugf("newvalue=%.10f | avg=%.10f", value, avg)

	ctrl.Publish(framework.TransducerPrefix+"/"+d.outtopics[index], fmt.Sprintf("%.10f", avg))
}

// run is the main function that gets called once form main()
func run(ctx *cli.Context) error {
	/* Set logging level (verbosity) */
	log.SetLevel(log.Level(uint32(ctx.Int("log-level"))))

	log.Info("Starting Math Avg Service")

	/* Start framework service client */
	c, err := framework.StartServiceClientManaged(
		ctx.String("framework-server"),
		ctx.String("mqtt-server"),
		ctx.String("service-id"),
		ctx.String("service-token"),
		"Unexpected disconnect!",
		NewDevice)
	if err != nil {
		log.Error("Failed to StartServiceClient: ", err)
		return cli.NewExitError(nil, 1)
	}
	defer c.StopClient()
	log.Info("Started service")

	/* Post service's global status */
	if err := c.SetStatus("Starting"); err != nil {
		log.Error("Failed to publish service status: ", err)
		return cli.NewExitError(nil, 1)
	}
	log.Info("Published Service Status")

	/* Setup signal channel */
	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	/* Post service status indicating I started */
	if err := c.SetStatus("Started"); err != nil {
		log.Error("Failed to publish service status: ", err)
		return cli.NewExitError(nil, 1)
	}
	log.Info("Published Service Status")

	/* Wait on a signal */
	sig := <-signals
	log.Info("Received signal ", sig)
	log.Warning("Shutting down")

	/* Post service's global status */
	if err := c.SetStatus("Shutting down"); err != nil {
		log.Error("Failed to publish service status: ", err)
	}
	log.Info("Published service status")

	return nil
}

func main() {
	/* Parse arguments and environmental variable */
	app := cli.NewApp()
	app.Name = "math-avg-service"
	app.Usage = ""
	app.Copyright = "See https://github.com/openchirp/math-avg-service for copyright information"
	app.Version = version
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "framework-server",
			Usage:  "OpenChirp framework server's URI",
			Value:  "http://localhost:7000",
			EnvVar: "FRAMEWORK_SERVER",
		},
		cli.StringFlag{
			Name:   "mqtt-server",
			Usage:  "MQTT server's URI (e.g. scheme://host:port where scheme is tcp or tls)",
			Value:  "tls://localhost:1883",
			EnvVar: "MQTT_SERVER",
		},
		cli.StringFlag{
			Name:   "service-id",
			Usage:  "OpenChirp service id",
			EnvVar: "SERVICE_ID",
		},
		cli.StringFlag{
			Name:   "service-token",
			Usage:  "OpenChirp service token",
			EnvVar: "SERVICE_TOKEN",
		},
		cli.IntFlag{
			Name:   "log-level",
			Value:  4,
			Usage:  "debug=5, info=4, warning=3, error=2, fatal=1, panic=0",
			EnvVar: "LOG_LEVEL",
		},
	}

	/* Launch the application */
	app.Run(os.Args)
}
