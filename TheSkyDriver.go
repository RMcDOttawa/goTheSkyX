package goTheSkyX

import (
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
)

// TheSkyDriver is the low-level interface to the TheSkyX application's TCP server, running
// somewhere on the network.  Controlling TheSky involves sending small packets of JavaScript
// to the server via a TCP socket.

type TheSkyDriver interface {
	// Basics
	Connect(server string, port int) error
	Close() error
	SetDebug(debug bool)
	SetVerbosity(verbosity int)
	// Camera
	ConnectCamera() error
	StartCooling(temp float64) error
	GetCameraTemperature() (float64, error)
	StopCooling() error
	// Frame Capture
	MeasureDownloadTime(binning int) (float64, error)
	StartDarkFrameCapture(binning int, seconds float64, downloadTime float64) error
	StartFlatFrameCapture(binning int, seconds float64, filterSlot int, downloadTime float64, saveImage bool) error
	IsCaptureDone() (bool, error)
	StartBiasFrameCapture(binning int, downloadTime float64) error
	GetADUValue() (int64, error)
	// Filters
	FilterWheelIsConnected() (bool, error)
	FilterWheelConnect() error
	FilterWheelDisconnect() error
	FilterNames() ([]string, error)
}

type TheSkyDriverInstance struct {
	isOpen          bool
	server          string
	port            int
	cameraConnected bool
	debug           bool
	verbosity       int
}

const FilterSlotNoFilter = -1

const maxTheSkyBuffer = 4096

// NewTheSkyDriver is the constructor for a working instance of the interface
func NewTheSkyDriver(
	debug bool, verbosity int) TheSkyDriver {
	driver := &TheSkyDriverInstance{
		debug:     debug,
		verbosity: verbosity,
	}
	return driver
}

func (driver *TheSkyDriverInstance) SetDebug(debug bool) {
	driver.debug = debug
}

func (driver *TheSkyDriverInstance) SetVerbosity(verbosity int) {
	driver.verbosity = verbosity
}

// Connect opens connection to the server and camera.
//
//	In fact, all we do is remember the server coordinates. The actual open of the
//	socket is deferred until we have a command to send
func (driver *TheSkyDriverInstance) Connect(server string, port int) error {
	if driver.verbosity >= 4 || driver.debug {
		fmt.Printf("TheSkyDriverInstance/Connect(%s,%d) entered\n", server, port)
	}
	if driver.isOpen {
		fmt.Printf("TheSkyDriverInstance/Connect(%s,%d): Already connected\n", server, port)
		return nil // already open, nothing to do
	}
	driver.server = server
	driver.port = port
	driver.isOpen = true
	if driver.verbosity >= 5 || driver.debug {
		fmt.Printf("TheSkyDriverInstance/Connect(%s,%d) successful\n", server, port)
	}
	return nil
}

// Close severs the connection to the TCP socket for the TheSkyX server
func (driver *TheSkyDriverInstance) Close() error {
	if driver.verbosity >= 4 || driver.debug {
		fmt.Printf("TheSkyDriverInstance/Close() entered\n")
	}
	if !driver.isOpen {
		fmt.Println("TheSkyDriverInstance/Close(): Not open")
		return nil
	}
	driver.isOpen = false
	if driver.verbosity >= 5 || driver.debug {
		fmt.Printf("TheSkyDriverInstance/Close() successful\n")
	}
	return nil
}

func (driver *TheSkyDriverInstance) ConnectCamera() error {
	if driver.verbosity >= 4 || driver.debug {
		fmt.Printf("TheSkyDriverInstance/ConnectCamera()  \n")
	}
	var commands strings.Builder
	commands.WriteString("ccdsoftCamera.Connect();\n")
	commands.WriteString("var Out;\n")
	commands.WriteString("Out=0;\n")

	if err := driver.sendCommandIgnoreReply(commands.String()); err != nil {
		fmt.Println("ConnectCamera error from driver:", err)
		return err
	}
	driver.cameraConnected = true
	return nil

}

// StartCooling sends server commands to turn on the TEC and set the target temperature
// No response is expected from these commands
func (driver *TheSkyDriverInstance) StartCooling(temperature float64) error {
	if driver.verbosity >= 4 || driver.debug {
		fmt.Printf("TheSkyDriverInstance/StartCooling(%g)  \n", temperature)
	}
	if !driver.cameraConnected {
		return errors.New("TheSkyDriverInstance/StartCooling: Camera not connected")
	}

	var commands strings.Builder
	commands.WriteString("ccdsoftCamera.RegulateTemperature=false;\n")
	commands.WriteString(fmt.Sprintf("ccdsoftCamera.TemperatureSetPoint=%.2f;\n", temperature))
	commands.WriteString("ccdsoftCamera.RegulateTemperature=true;\n")
	commands.WriteString("ccdsoftCamera.ShutDownTemperatureRegulationOnDisconnect=false;\n")

	if err := driver.sendCommandIgnoreReply(commands.String()); err != nil {
		fmt.Println("StartCooling error from driver:", err)
		return err
	}
	return nil
}

func (driver *TheSkyDriverInstance) StopCooling() error {
	var commands strings.Builder
	commands.WriteString("ccdsoftCamera.RegulateTemperature=false;\n")
	if !driver.cameraConnected {
		return errors.New("TheSkyDriverInstance/StopCooling: Camera not connected")
	}

	if err := driver.sendCommandIgnoreReply(commands.String()); err != nil {
		fmt.Println("StopCooling error from driver:", err)
		return err
	}
	return nil
}

// GetCameraTemperature polls TheSkyX for the current camera temperature and returns it
func (driver *TheSkyDriverInstance) GetCameraTemperature() (float64, error) {
	if driver.verbosity >= 4 || driver.debug {
		fmt.Println("GetCameraTemperature()")
	}
	if !driver.cameraConnected {
		return 0.0, errors.New("TheSkyDriverInstance/GetCameraTemperature: Camera not connected")
	}
	var commands strings.Builder
	commands.WriteString("var temp=ccdsoftCamera.Temperature;\n")
	commands.WriteString("var Out;\n")
	commands.WriteString("Out=temp + \"\\n\";\n")

	numberResult, err := driver.sendCommandFloatReply(commands.String())
	if err != nil {
		fmt.Println("GetCameraTemperature error from driver:", err)
		return -1.0, err
	}
	return numberResult, nil
}

func (driver *TheSkyDriverInstance) GetADUValue() (int64, error) {
	if driver.verbosity >= 4 || driver.debug {
		fmt.Println("GetADUValue()")
	}
	if !driver.cameraConnected {
		return 0.0, errors.New("TheSkyDriverInstance/GetADUValue: Camera not connected")
	}
	var commands strings.Builder
	commands.WriteString("ccdsoftCameraImage.AttachToActive();\n")
	commands.WriteString("var averageAdu = ccdsoftCameraImage.averagePixelValue();\n")
	commands.WriteString("var Out;\n")
	commands.WriteString("Out=averageAdu + \"\\n\";\n")

	numberResult, err := driver.sendCommandFloatReply(commands.String())
	if err != nil {
		fmt.Println("GetCameraTemperature error from driver:", err)
		return -1.0, err
	}
	return int64(math.Round(numberResult)), nil
}

// MeasureDownloadTime measures the time needed to download an image from the camera to the TheSkyX application
// We do this because the download time is often significant, especially on older cameras, and because TheSkyX
// does not provide a notification that download is complete. By knowing the download time, we can initiate an exposure
// and then wait (exposure time + download time) before polling if the camera is done.
//
// Download time is a function of the binning level used - higher binning means fewer pixels to download, so faster.
//
// To measure the download time, we direct the server to capture a 0.1-second exposure at the given binning level.
// A zero-length bias frame would be better, but not all cameras support this, so we settle for the 0.1 dark.
// We will have the server note the time before and after this exposure, and return the two numbers to us.
// Then we calculate the download time as the difference minus the 0.1 second of the exposure itself.

// Here is the javascript we will use, explained:
//	 // Prepare
//	 ccdsoftCamera.Autoguider=false;    			// Use main camera not autoguider
//	 ccdsoftCamera.Asynchronous=false;  			// synchronous (i.e., wait)
//	 ccdsoftCamera.Frame=3;  						// Type "3" is dark frame
//	 ccdsoftCamera.ImageReduction=0;				// Don't reduce the image
//	 ccdsoftCamera.ToNewWindow=false;				// Don't open a new window with the image
//	 ccdsoftCamera.ccdsoftAutoSaveAs=0;				// Don't save the image to disk
//	 ccdsoftCamera.AutoSaveOn=false;				// Don't save the image to disk
//	 ccdsoftCamera.BinX=1;							// Set the binning level
//	 ccdsoftCamera.BinY=1;							// Set the binning level
//	 ccdsoftCamera.ExposureTime=0.1;				// Set the exposure time
//
//	 // Record the time before the image
//	 sky6Utils.ComputeUniversalTime();				// Put current time in dOut0 variable (theSKyX weirdness)
//	 var timeBefore=sky6Utils.dOut0;
//
//	 // Take and download the image
//	 var cameraResult = ccdsoftCamera.TakeImage();	// Take the image
//
//	 // Record the time after the image
//	 sky6Utils.ComputeUniversalTime();
//	 var timeAfter=sky6Utils.dOut0;
//
//	 // Return the before and after times
//	 var out;
//	 out = timeBefore + "," + timeAfter + "\n";

const shortExposureLength = 0.1

func (driver *TheSkyDriverInstance) MeasureDownloadTime(binning int) (float64, error) {
	if driver.verbosity >= 4 || driver.debug {
		fmt.Println("TheSkyDriverInstance/MeasureDownloadTime ", binning)
	}
	if !driver.cameraConnected {
		return 0.0, errors.New("TheSkyDriverInstance/MeasureDownloadTime: Camera not connected")
	}
	var message strings.Builder
	message.WriteString("ccdsoftCamera.Autoguider=false;\n")
	message.WriteString("ccdsoftCamera.Asynchronous=false;\n")
	message.WriteString("ccdsoftCamera.Frame=3;\n")
	message.WriteString("ccdsoftCamera.ImageReduction=0;\n")
	message.WriteString("ccdsoftCamera.ToNewWindow=false;\n")
	message.WriteString("ccdsoftCamera.ccdsoftAutoSaveAs=0;\n")
	message.WriteString("ccdsoftCamera.AutoSaveOn=false;\n")
	message.WriteString("ccdsoftCamera.BinX=1;\n")
	message.WriteString("ccdsoftCamera.BinY=1;\n")
	message.WriteString(fmt.Sprintf("ccdsoftCamera.ExposureTime=%.2f;\n", shortExposureLength))
	message.WriteString("sky6Utils.ComputeUniversalTime();\n")
	message.WriteString("var timeBefore=sky6Utils.dOut0;\n")
	message.WriteString("var cameraResult = ccdsoftCamera.TakeImage();\n")
	message.WriteString("sky6Utils.ComputeUniversalTime();\n")
	message.WriteString("var timeAfter=sky6Utils.dOut0;\n")
	message.WriteString("var out;\n")
	message.WriteString("out = timeBefore + \",\" + timeAfter + \"\\n\";\n")
	//fmt.Println("Command to send:\n", message.String())

	responseString, err := driver.sendCommandStringReply(message.String())
	if err != nil {
		fmt.Println("MeasureDownloadTime error from driver:", err)
		return -1.0, err
	}
	responseParts := strings.Split(responseString, ",")

	timeBefore, err := strconv.ParseFloat(responseParts[0], 64)
	if err != nil {
		return -1.0, errors.New("error parsing timeBefore")
	}
	timeAfter, err := strconv.ParseFloat(responseParts[1], 64)
	if err != nil {
		return -1.0, errors.New("error parsing timeAfter")
	}

	//	Edge case.  If timeAfter is less than timeBefore, it means the day changed during the exposure
	//	We will add 24 hours to timeAfter to correct this
	if timeAfter < timeBefore {
		timeAfter += 24.0
	}

	secondsTaken := (timeAfter-timeBefore)*60.0*60.0 - shortExposureLength

	return secondsTaken, nil
}

func (driver *TheSkyDriverInstance) StartDarkFrameCapture(binning int, seconds float64, downloadTime float64) error {
	if driver.verbosity >= 4 || driver.debug {
		fmt.Println("TheSkyDriverInstance/StartDarkFrameCapture ", binning, seconds, downloadTime)
	}
	if !driver.cameraConnected {
		return errors.New("TheSkyDriverInstance/StartDarkFrameCapture: Camera not connected")
	}
	var message strings.Builder
	message.WriteString("ccdsoftCamera.Autoguider=false;\n")  // Use main camera not autoguider
	message.WriteString("ccdsoftCamera.Asynchronous=true;\n") // Async (don't wait)
	message.WriteString("ccdsoftCamera.Frame=3;\n")           // Dark frame
	message.WriteString("ccdsoftCamera.ImageReduction=0;\n")  // No image reduction
	message.WriteString("ccdsoftCamera.ToNewWindow=false;\n") // Don't open a new window
	message.WriteString("ccdsoftCamera.AutoSaveOn=true;\n")   // Save the image to configured location
	message.WriteString(fmt.Sprintf("ccdsoftCamera.BinX=%d;\n", binning))
	message.WriteString(fmt.Sprintf("ccdsoftCamera.BinY=%d;\n", binning))
	message.WriteString(fmt.Sprintf("ccdsoftCamera.ExposureTime=%.2f;\n", seconds))
	message.WriteString("var cameraResult = ccdsoftCamera.TakeImage();\n")
	message.WriteString("var Out;\n")
	message.WriteString("Out=cameraResult+\"\\n\";\n")

	err := driver.sendCommandIgnoreReply(message.String())
	if err != nil {
		fmt.Println("CaptureDarkFrame error from driver on starting capture:", err)
		return err
	}
	//fmt.Println("Camera response:", response)
	return nil
}

// sendCommandIgnoreReply is an internal method that sends the given command string to the server.
// This is used for commands where no reply is to be read and processed by the caller
// (There is a reply from the server, but it is used only to verify successful execution)
func (driver *TheSkyDriverInstance) sendCommandIgnoreReply(command string) error {
	if driver.verbosity >= 6 || driver.debug {
		fmt.Println("TheSkyDriverInstance/sendCommandIgnoreReply: ", command)
	}
	var message strings.Builder
	message.WriteString("/* Java Script */\n")
	message.WriteString("/* Socket Start Packet */\n")
	message.WriteString(command)
	message.WriteString("/* Socket End Packet */\n")

	response, err := driver.sendCommand(message.String())
	if driver.verbosity >= 6 || driver.debug {
		fmt.Println("TheSkyDriverInstance/sendCommandIgnoreReply ignoring response: ", response)
	}
	if err != nil {
		fmt.Println("sendCommandNoReply error from driver:", err)
		return err
	}
	return nil
}

// sendCommandFloatReply is an internal method that sends the given command string to the server.
// This is used for commands where a floating point number reply is to be read and processed by the caller
func (driver *TheSkyDriverInstance) sendCommandFloatReply(command string) (float64, error) {
	if driver.verbosity >= 6 || driver.debug {
		fmt.Println("TheSkyDriverInstance/sendCommandFloatReply: ", command)
	}

	var message strings.Builder
	message.WriteString("/* Java Script */\n")
	message.WriteString("/* Socket Start Packet */\n")
	message.WriteString(command)
	message.WriteString("/* Socket End Packet */\n")

	responseString, err := driver.sendCommand(message.String())
	trimmedResponse := strings.TrimSpace(responseString)
	if err != nil {
		fmt.Println("sendCommandNoReply error from driver:", err)
		return 0.0, err
	}

	parsedNum, err := strconv.ParseFloat(trimmedResponse, 64)
	if err != nil {
		return parsedNum, errors.New("error parsing numeric result")
	}

	return parsedNum, nil
}

// sendCommandStringReply is an internal method that sends the given command string to the server.
// This is used for commands where an arbitrary string reply is to be read and processed by the caller
func (driver *TheSkyDriverInstance) sendCommandStringReply(command string) (string, error) {
	if driver.verbosity >= 6 || driver.debug {
		fmt.Println("TheSkyDriverInstance/sendCommandStringReply: ", command)
	}

	var message strings.Builder
	message.WriteString("/* Java Script */\n")
	message.WriteString("/* Socket Start Packet */\n")
	message.WriteString(command)
	message.WriteString("/* Socket End Packet */\n")

	responseString, err := driver.sendCommand(message.String())
	trimmedResponse := strings.TrimSpace(responseString)
	if err != nil {
		fmt.Println("sendCommandNoReply error from driver:", err)
		return "", err
	}

	return trimmedResponse, nil
}

// sendCommand is an internal method that sends the given command packet to the server and
// returns whatever reply is received.
func (driver *TheSkyDriverInstance) sendCommand(command string) (string, error) {
	//fmt.Println("TheSkyDriverInstance/sendCommand:", command)
	//	This function must be mutex-locked in case of parallel activities
	var mutex sync.Mutex
	mutex.Lock()
	defer mutex.Unlock()

	if driver.verbosity >= 6 || driver.debug {
		fmt.Printf("TheSkyDriverInstance/sendCommand() opening socket(%s,%d)\n", driver.server, driver.port)
	}
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", driver.server, driver.port))
	if err != nil {
		fmt.Println("Error opening socket:", err)
		return "", err
	}
	defer func(conn net.Conn) {
		if driver.verbosity > 4 || driver.debug {
			fmt.Println("Closing socket")
		}
		_ = conn.Close()
	}(conn)

	numWritten, err := conn.Write([]byte(command))
	if err != nil {
		fmt.Println("sendCommand error from driver:", err)
		return "", err
	}
	if numWritten != len(command) {
		fmt.Println("sendCommand wrong number of bytes from driver")
		return "", errors.New("sendCommand wrong number of bytes from driver")
	}

	responseBuffer := make([]byte, maxTheSkyBuffer)
	numRead, err := conn.Read(responseBuffer)
	if err != nil {
		fmt.Println("sendCommand error from driver:", err)
		return "", err
	}
	if driver.verbosity >= 5 || driver.debug {
		fmt.Println("TheSkyDriverInstance/sendCommand() received response:", string(responseBuffer[:numRead]))
	}

	//	Response will be of the form <data if any> | error line
	responseParts := strings.Split(string(responseBuffer[:numRead]), "|")
	responseText := responseParts[0]
	errorLine := strings.ToLower(responseParts[1])

	if errorLine == "" {
		return responseText, nil
	}
	if strings.HasPrefix(errorLine, "no error.") {
		return responseText, nil
	}
	return responseText, errors.New("TheSkyX error: " + errorLine)
}

// IsCaptureDone polls the server to see if the camera is done with its current activity
func (driver *TheSkyDriverInstance) IsCaptureDone() (bool, error) {
	if driver.verbosity >= 5 || driver.debug {
		fmt.Println("TheSkyDriverInstance/IsCaptureDone()")
	}
	if !driver.cameraConnected {
		return false, errors.New("TheSkyDriverInstance/IsCaptureDone: Camera not connected")
	}
	var message strings.Builder
	message.WriteString("var complete = ccdsoftCamera.IsExposureComplete;\n")
	message.WriteString("var Out;\n")
	message.WriteString("Out=complete+\"\\n\";\n")

	responseString, err := driver.sendCommandStringReply(message.String())
	if err != nil {
		fmt.Println("IsCaptureDone error from driver IsExposureComplete:", err)
		return false, err
	}
	if driver.verbosity >= 5 || driver.debug {
		fmt.Println("IsCaptureDone response:", responseString)
	}
	return responseString == "1", nil
}

func (driver *TheSkyDriverInstance) StartBiasFrameCapture(binning int, downloadTime float64) error {
	if driver.verbosity >= 4 || driver.debug {
		fmt.Println("TheSkyDriverInstance/StartBiasFrameCapture ", binning, downloadTime)
	}
	if !driver.cameraConnected {
		return errors.New("TheSkyDriverInstance/StartBiasFrameCapture: Camera not connected")
	}
	var message strings.Builder
	message.WriteString("ccdsoftCamera.Autoguider=false;\n")  // Use main camera not autoguider
	message.WriteString("ccdsoftCamera.Asynchronous=true;\n") // Async (don't wait)
	message.WriteString("ccdsoftCamera.Frame=2;\n")           // Bias frame
	message.WriteString("ccdsoftCamera.ImageReduction=0;\n")  // No image reduction
	message.WriteString("ccdsoftCamera.ToNewWindow=false;\n") // Don't open a new window
	message.WriteString("ccdsoftCamera.AutoSaveOn=true;\n")   // Save the image to configured location
	message.WriteString(fmt.Sprintf("ccdsoftCamera.BinX=%d;\n", binning))
	message.WriteString(fmt.Sprintf("ccdsoftCamera.BinY=%d;\n", binning))
	message.WriteString("var cameraResult = ccdsoftCamera.TakeImage();\n")
	message.WriteString("var Out;\n")
	message.WriteString("Out=cameraResult+\"\\n\";\n")

	err := driver.sendCommandIgnoreReply(message.String())
	if err != nil {
		fmt.Println("StartBiasFrameCapture error from driver on starting capture:", err)
		return err
	}
	//fmt.Println("Camera response:", response)
	return nil
}

func (driver *TheSkyDriverInstance) StartFlatFrameCapture(binning int, seconds float64, filterSlot int, downloadTime float64, saveImage bool) error {
	if driver.verbosity >= 4 || driver.debug {
		fmt.Println("TheSkyDriverInstance/StartFlatFrameCapture ", binning, seconds, downloadTime)
	}
	if !driver.cameraConnected {
		return errors.New("TheSkyDriverInstance/StartFlatFrameCapture: Camera not connected")
	}
	var message strings.Builder
	if filterSlot != FilterSlotNoFilter {
		message.WriteString("ccdsoftCamera.filterWheelConnect();\n")
		// Note: filter slot is zero-based so we subtract one
		message.WriteString(fmt.Sprintf("ccdsoftCamera.FilterIndexZeroBased=%d;\n", filterSlot-1))
	}
	message.WriteString("ccdsoftCamera.Autoguider=false;\n")                                          // Use main camera not autoguider
	message.WriteString("ccdsoftCamera.Asynchronous=true;\n")                                         // Async (don't wait)
	message.WriteString("ccdsoftCamera.Frame=4;\n")                                                   // Flat frame
	message.WriteString("ccdsoftCamera.ImageReduction=0;\n")                                          // No image reduction
	message.WriteString("ccdsoftCamera.ToNewWindow=false;\n")                                         // Don't open a new window
	message.WriteString(fmt.Sprintf("ccdsoftCamera.AutoSaveOn=%s;\n", makeJavascriptBool(saveImage))) // Save the image?
	message.WriteString(fmt.Sprintf("ccdsoftCamera.BinX=%d;\n", binning))
	message.WriteString(fmt.Sprintf("ccdsoftCamera.BinY=%d;\n", binning))
	message.WriteString(fmt.Sprintf("ccdsoftCamera.ExposureTime=%.2f;\n", seconds))
	message.WriteString("var cameraResult = ccdsoftCamera.TakeImage();\n")
	message.WriteString("var Out;\n")
	message.WriteString("Out=cameraResult+\"\\n\";\n")

	err := driver.sendCommandIgnoreReply(message.String())
	if err != nil {
		fmt.Println("StartFlatFrameCapture error from driver on starting capture:", err)
		return err
	}
	return nil
}

func makeJavascriptBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func (driver *TheSkyDriverInstance) sendCommandIntReply(s string) (int, error) {
	response, err := driver.sendCommandStringReply(s)
	if err != nil {
		return 0, err
	}
	response = strings.TrimSpace(response)
	result, err := strconv.Atoi(response)
	if err != nil {
		return 0, errors.New(fmt.Sprintf("sendCommandIntReply: error parsing numeric result: %s", err))
	}
	return result, nil
}

func (driver *TheSkyDriverInstance) FilterWheelIsConnected() (bool, error) {
	//fmt.Println("FilterWheelIsConnected")
	var message strings.Builder
	message.WriteString("var isConnected;\n")
	message.WriteString("isConnected = ccdsoftCamera.filterWheelIsConnected();\n")
	message.WriteString("var out;\n")
	message.WriteString("out = isConnected + \"\\n\";\n")
	//fmt.Println("Command to send:\n", message.String())

	responseCode, err := driver.sendCommandIntReply(message.String())
	if err != nil {
		//fmt.Println("HasFilterWheel error from driver:", err)
		return false, err
	}

	//fmt.Println("HasFilterWheel response:", responseCode)

	return responseCode == 1, nil
}

func (driver *TheSkyDriverInstance) FilterWheelConnect() error {
	//fmt.Println("FilterWheelConnect ")
	var message strings.Builder
	message.WriteString("result = ccdsoftCamera.filterWheelConnect();\n")
	message.WriteString("var out;\n")
	message.WriteString("out = result + \"\\n\";\n")
	//fmt.Println("Command to send:\n", message.String())

	responseCode, err := driver.sendCommandIntReply(message.String())
	if err != nil {
		//fmt.Println("HasFilterWheel error from driver:", err)
		return err
	}
	if responseCode != 0 {
		return errors.New(fmt.Sprintf("FilterWheelConnect: response code %d", responseCode))
	}

	return nil
}

func (driver *TheSkyDriverInstance) FilterWheelDisconnect() error {
	fmt.Println("FilterWheelDisconnect STUB")
	return errors.New("FilterWheelDisconnect not implemented yet")
}

//	Retrieve a list of filter names from the camera
//	There is no single command to do this, so we send a script that
//		Gets the number of filters
//		Loops through indexes up to that number, retrieving filter by slot number
//		Results are concatenated together with tab characters and returned in a single string
//
//	The following javascript
//		var numFilters = ccdsoftCamera.lNumberFilters;
//		var result = "";
//		var i;
//		for (i = 0; i < numFilters; i++) {
//			filterName = ccdsoftCamera.szFilterName(i);
//		    result = result + "\t" + filterName;
//		}
//		var out = result + "\n";

func (driver *TheSkyDriverInstance) FilterNames() ([]string, error) {
	//fmt.Println("FilterNames STUB")
	var message strings.Builder
	message.WriteString("var numFilters = ccdsoftCamera.lNumberFilters;\n")
	message.WriteString("var result = \"\";\n")
	message.WriteString("var i;\n")
	message.WriteString("for (i = 0; i < numFilters; i++) {\n")
	message.WriteString("   filterName = ccdsoftCamera.szFilterName(i);\n")
	message.WriteString("   result = result + \"\\t\" + filterName;\n")
	message.WriteString("}\n")
	message.WriteString("var out = result + \"\\n\";\n")

	//fmt.Println("Command to send:\n", message.String())

	responseBlob, err := driver.sendCommandStringReply(message.String())
	if err != nil {
		fmt.Println("FilterNames error from driver:", err)
		return []string{}, err
	}

	//fmt.Println("FilterNames response:", responseBlob)
	filterNames := strings.Split(responseBlob, "\t")
	return filterNames, nil
}

//func (driver *TheSkyDriverInstance) xxxxx(args) error {
//}

//var message strings.Builder
//message.WriteString("var isConnected;\n")
//message.WriteString("isConnected = ccdsoftCamera.filterWheelIsConnected();\n")
//message.WriteString("var out;\n")
//message.WriteString("out = isConnected + \"\\n\";\n")
//fmt.Println("Command to send:\n", message.String())
//
//responseCode, err := driver.sendCommandIntReply(message.String())
//if err != nil {
//	fmt.Println("HasFilterWheel error from driver:", err)
//	return false, err
//}
//
//fmt.Println("HasFilterWheel response:", responseCode)
//
//return false, nil
