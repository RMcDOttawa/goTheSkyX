package goTheSkyX

import (
	"errors"
	"fmt"
	"github.com/RMcDOttawa/goMockableDelay"
	"math"
	"math/rand/v2"
	"runtime/debug"
	"strings"
	"time"
)

//	TheSkyService is a high-level interface to the set of logical services we use to control
//	the TheSkyX app running on the network. It abstracts away the complexities of making up
//	JavaScript command packets and using sockets to communicate.

type TheSkyService interface {
	//	BAsic Controls
	Connect(server string, port int) error
	Close() error
	SetDriver(driver TheSkyDriver)
	SetDebug(debug bool)
	SetVerbosity(verbosity int)
	//	Camera
	ConnectCamera() error
	StartCooling(targetTemp float64) error
	GetCameraTemperature() (float64, error)
	StopCooling() error
	WaitForCameraInactive(pollingIntervalSeconds int, timeoutMinutes int) error
	//	Filter Wheel
	HasFilterWheel() (bool, error)
	NumberOfFilters() (int, error)  // Number of up to first blank name
	FilterNames() ([]string, error) // Names up to first blank name
	//	Frame Capture
	MeasureDownloadTime(binning int) (float64, error)
	CaptureDarkFrame(binning int, seconds float64, downloadTime float64) error
	CaptureBiasFrame(binning int, downloadTime float64) error // for mocking
	CaptureAndMeasureFlatFrame(exposure float64, binning int, filterSlot int, downloadTime float64, saveImage bool) (int64, error)
	SetSimulateFlatCapture(flag bool)
	SetSimulationNoiseFraction(fraction float64)
}

type TheSkyServiceInstance struct {
	driver                  TheSkyDriver
	isOpen                  bool
	delayService            goMockableDelay.DelayService
	debug                   bool
	verbosity               int
	simulateFlatCapture     bool
	simulationNoiseFraction float64
}

const minimumTimeoutForDark = 10.0 * 60.0
const minimumTimeoutForBias = 3.0 * 60.0

//const simulationNoiseFraction = 0.0

func (service *TheSkyServiceInstance) SetDriver(driver TheSkyDriver) {
	service.driver = driver
}

func (service *TheSkyServiceInstance) SetDebug(debug bool) {
	service.debug = debug
}

func (service *TheSkyServiceInstance) SetVerbosity(verbosity int) {
	service.verbosity = verbosity
}

func (service *TheSkyServiceInstance) SetSimulateFlatCapture(flag bool) {
	service.simulateFlatCapture = flag
}

func (service *TheSkyServiceInstance) SetSimulationNoiseFraction(fraction float64) {
	service.simulationNoiseFraction = fraction
}

// NewTheSkyService is the constructor for the instance of this service
func NewTheSkyService(delayService goMockableDelay.DelayService,
	debug bool,
	verbosity int,
	simulateFlatFrameADUs bool) TheSkyService {
	service := &TheSkyServiceInstance{
		isOpen:                  false,
		driver:                  NewTheSkyDriver(debug, verbosity),
		delayService:            delayService,
		debug:                   debug,
		verbosity:               verbosity,
		simulateFlatCapture:     simulateFlatFrameADUs,
		simulationNoiseFraction: 0.2,
	}
	return service
}

// Connect opens a connection to the TheSkyX application, via the low-level driver.
// The connection is kept open, ready to use.
func (service *TheSkyServiceInstance) Connect(server string, port int) error {
	//fmt.Printf("TheSkyServiceInstance/Connect(%s,%d)\n", server, port)
	if service.isOpen {
		fmt.Printf("TheSkyServiceInstance/Connect(%s,%d): Already connected\n", server, port)
		return nil // already open, nothing to do
	}

	if err := service.driver.Connect(server, port); err != nil {
		return err
	}
	service.isOpen = true

	if err := service.ConnectCamera(); err != nil {
		fmt.Println("Error in TheSkyServiceInstance/Connect, connecting camera:", err)
		return err
	}

	return nil
}

// ConnectCamera asks TheSky to connect to the camera.
func (service *TheSkyServiceInstance) ConnectCamera() error {
	//fmt.Printf("TheSkyServiceInstance/ConnectCamera()\n")
	if !service.isOpen {
		return errors.New("TheSkyServiceInstance/ConnectCamera: Connection not open")
	}
	err := service.driver.ConnectCamera()
	if err != nil {
		fmt.Println("TheSkyServiceInstance/ConnectCamera error from driver:", err)
		return err
	}
	return nil
}

func (service *TheSkyServiceInstance) WaitForCameraInactive(pollingIntervalSeconds int, timeoutMinutes int) error {
	if service.verbosity >= 5 {
		fmt.Printf("TheSkyServiceInstance/WaitForCameraInactive(%d)\n", pollingIntervalSeconds)
	}
	if !service.isOpen {
		return errors.New("TheSkyServiceInstance/WaitForCameraInactive: Connection not open")
	}
	err := service.driver.ConnectCamera()
	if err != nil {
		fmt.Println("TheSkyServiceInstance/ConnectCamera error from driver:", err)
		return err
	}
	timeoutTime := time.Now().Add(time.Duration(timeoutMinutes) * time.Minute)
	for {
		done, err := service.driver.IsCaptureDone()
		if err != nil {
			fmt.Println("TheSkyServiceInstance/WaitForCameraInactive error from IsCaptureDone:", err)
			return err
		}
		if done {
			if service.verbosity >= 5 {
				fmt.Println("  Camera done, returning")
			}
			break
		}
		if service.verbosity >= 5 {
			fmt.Printf("  Camera not done, waiting %d seconds to try again\n", pollingIntervalSeconds)
		}
		_, err = service.delayService.DelayDuration(pollingIntervalSeconds)
		if time.Now().After(timeoutTime) {
			return errors.New("timed out waiting for camera to finish")
		}
	}
	return nil
}

// Close closes the connection to the TheSkyX server
func (service *TheSkyServiceInstance) Close() error {
	//fmt.Println("TheSkyServiceInstance/Close() ")
	if !service.isOpen {
		fmt.Println("TheSkyServiceInstance/Close(): Not open")
		return nil
	}

	if err := service.driver.Close(); err != nil {
		return err
	}
	service.isOpen = false
	return nil
}

// StartCooling turns on the camera's thermoelectric cooler (TEC) and sets target temp
func (service *TheSkyServiceInstance) StartCooling(targetTemp float64) error {
	if service.debug || service.verbosity >= 4 {
		fmt.Printf("TheSkyServiceInstance/startCooling(%g) entered\n", targetTemp)
	}
	if !service.isOpen {
		return errors.New("TheSkyServiceInstance/StartCooling: Connection not open")
	}

	if err := service.driver.StartCooling(targetTemp); err != nil {
		fmt.Println("TheSkyServiceInstance/StartCooling error from driver:", err)
		return err
	}
	if service.debug || service.verbosity >= 4 {
		fmt.Printf("TheSkyServiceInstance/startCooling(%g) exits\n", targetTemp)
	}
	return nil
}

func (service *TheSkyServiceInstance) StopCooling() error {
	//fmt.Println("TheSkyServiceInstance/StopCooling()")
	if !service.isOpen {
		return errors.New("TheSkyServiceInstance/StopCooling: Connection not open")
	}
	err := service.driver.StopCooling()
	if err != nil {
		fmt.Println("TheSkyServiceInstance/StopCooling error from driver:", err)
		return err
	}
	return nil

}

func (service *TheSkyServiceInstance) GetCameraTemperature() (float64, error) {
	//fmt.Println("TheSkyServiceInstance/GetCameraTemperature()")
	if !service.isOpen {
		return 0.0, errors.New("TheSkyServiceInstance/GetCameraTemperature: Connection not open")
	}
	temp, err := service.driver.GetCameraTemperature()
	if err != nil {
		fmt.Println("TheSkyServiceInstance/GetCameraTemperature error from driver:", err)
		return temp, err
	}
	return temp, nil
}

func (service *TheSkyServiceInstance) MeasureDownloadTime(binning int) (float64, error) {
	if !service.isOpen {
		return 0.0, errors.New("TheSkyServiceInstance/MeasureDownloadTime: Connection not open")
	}
	downloadTime, err := service.driver.MeasureDownloadTime(binning)
	if err != nil {
		fmt.Println("TheSkyServiceInstance/MeasureDownloadTime error from driver:", err)
		return downloadTime, err
	}
	return downloadTime, nil
}

const AndALittleExtra = 0.5
const pollingInterval = 2.0 //	seconds between polls
const timeoutFactor = 5.0   // How much longer to wait than the exposure time
const shortTimeForBiasExposure = 0.1

func (service *TheSkyServiceInstance) CaptureDarkFrame(binning int, seconds float64, downloadTime float64) error {
	if service.verbosity >= 4 || service.debug {
		fmt.Printf("TheSkyServiceInstance/CaptureDarkFrame(%d, %g, %g) \n", binning, seconds, downloadTime)
	}
	err := service.driver.StartDarkFrameCapture(binning, seconds, downloadTime)
	if err != nil {
		fmt.Println("TheSkyServiceInstance/StartDarkFrameCapture error from driver:", err)
		return err
	}
	//	Now we'll wait until the exposure is probably over - exposure time + download time
	delayUntilComplete := int(math.Round(seconds + downloadTime + AndALittleExtra))
	if service.verbosity >= 4 {
		fmt.Println("Exposure started. Waiting for ", delayUntilComplete)
	}
	if _, err := service.delayService.DelayDuration(delayUntilComplete); err != nil {
		fmt.Println("TheSkyServiceInstance/CaptureDarkFrame error from delaypkg service:", err)
		return err
	}
	//	Now we poll the camera repeatedly until it reports done
	maximumWaitSeconds := math.Max((seconds+downloadTime)*timeoutFactor, minimumTimeoutForDark)
	secondsWaitedSoFar := 0.0
	for {
		done, err := service.driver.IsCaptureDone()
		if err != nil {
			fmt.Println("TheSkyServiceInstance/CaptureDarkFrame error from IsCaptureDone:", err)
			return err
		}
		if done {
			if service.verbosity >= 4 {
				fmt.Println("capture is done, returning")
			}
			return nil
		}
		if secondsWaitedSoFar > maximumWaitSeconds {
			return errors.New("TheSkyServiceInstance/CaptureDarkFrame: Timeout waiting for capture to finish")
		}
		if service.verbosity >= 4 {
			fmt.Println("Camera not finished. Delaying ", pollingInterval)
		}
		if _, err := service.delayService.DelayDuration(int(math.Round(pollingInterval))); err != nil {
			fmt.Println("TheSkyServiceInstance/CaptureDarkFrame error from polling delaypkg service:", err)
			return err
		}
		secondsWaitedSoFar += pollingInterval
	}
}

func (service *TheSkyServiceInstance) CaptureBiasFrame(binning int, downloadTime float64) error {
	if service.verbosity >= 4 || service.debug {
		fmt.Printf("TheSkyServiceInstance/CaptureBiasFrame(%d, %g) \n", binning, downloadTime)
	}
	err := service.driver.StartBiasFrameCapture(binning, downloadTime)
	if err != nil {
		fmt.Println("TheSkyServiceInstance/StartBiasFrameCapture error from driver:", err)
		return err
	}
	//	Now we'll wait until the exposure is probably over - exposure time + download time
	delayUntilComplete := int(math.Round(shortTimeForBiasExposure + downloadTime + AndALittleExtra))
	if service.verbosity >= 4 {
		fmt.Println("Exposure started. Waiting for ", delayUntilComplete)
	}
	if _, err := service.delayService.DelayDuration(delayUntilComplete); err != nil {
		fmt.Println("TheSkyServiceInstance/CaptureBiasFrame error from delaypkg service:", err)
		return err
	}
	//	Now we poll the camera repeatedly until it reports done
	maximumWaitSeconds := math.Max((shortTimeForBiasExposure+downloadTime)*timeoutFactor, minimumTimeoutForBias)
	secondsWaitedSoFar := 0.0
	for {
		done, err := service.driver.IsCaptureDone()
		if err != nil {
			fmt.Println("TheSkyServiceInstance/CaptureBiasFrame error from IsCaptureDone:", err)
			return err
		}
		if done {
			if service.verbosity >= 4 {
				fmt.Println("capture is done, returning")
			}
			return nil
		}
		if secondsWaitedSoFar > maximumWaitSeconds {
			return errors.New("TheSkyServiceInstance/CaptureBiasFrame: Timeout waiting for capture to finish")
		}
		if service.verbosity >= 4 {
			fmt.Println("Camera not finished. Delaying ", pollingInterval)
		}
		if _, err := service.delayService.DelayDuration(int(math.Round(pollingInterval))); err != nil {
			//if _, err := service.delayService.DelayDuration(10); err != nil {
			fmt.Println("TheSkyServiceInstance/CaptureBiasFrame error from polling delaypkg service:", err)
			return err
		}
		secondsWaitedSoFar += pollingInterval
	}
}

// Note that testing this function without a live camera and a real flat target is difficult, as
// the ADU value returned will not be typical.  (From TheSkyX's camera simulator, it is a constant value).
// So, we have an optional testing simulator that can return ADUs empirically calculated from testing.
// This, if used, is run after the driver runs the CaptureFlat routine so we still exercise the driver and the waiting

func (service *TheSkyServiceInstance) CaptureAndMeasureFlatFrame(exposure float64, binning int, filterSlot int, downloadTime float64, saveImage bool) (int64, error) {
	if service.verbosity >= 4 || service.debug {
		fmt.Printf("TheSkyServiceInstance/CaptureAndMeasureFlatFrame(%g, %d, %g, %t) \n", exposure, binning, downloadTime, saveImage)
	}
	if exposure == 0.0 {
		fmt.Printf("TheSkyServiceInstance/CaptureAndMeasureFlatFrame ASSERT FAIL, exposure 0 (%g, %d, %g, %t) \n", exposure, binning, downloadTime, saveImage)
		debug.PrintStack()
		panic("Exposure=0")
	}
	err := service.driver.StartFlatFrameCapture(binning, exposure, filterSlot, downloadTime, saveImage)
	if err != nil {
		fmt.Println("TheSkyServiceInstance/StartFlatFrameCapture error from driver:", err)
		return 0, err
	}
	//	Now we'll wait until the exposure is probably over - exposure time + download time
	delayUntilComplete := int(math.Round(exposure + downloadTime + AndALittleExtra))
	if service.verbosity >= 4 {
		fmt.Println("Exposure started. Waiting for ", delayUntilComplete)
	}
	if _, err := service.delayService.DelayDuration(delayUntilComplete); err != nil {
		fmt.Println("TheSkyServiceInstance/CaptureAndMeasureFlatFrame error from delaypkg service:", err)
		return 0, err
	}
	//	Now we poll the camera repeatedly until it reports done
	maximumWaitSeconds := math.Max((exposure+downloadTime)*timeoutFactor, minimumTimeoutForDark)
	secondsWaitedSoFar := 0.0
	for {
		done, err := service.driver.IsCaptureDone()
		if err != nil {
			fmt.Println("TheSkyServiceInstance/CaptureAndMeasureFlatFrame error from IsCaptureDone:", err)
			return 0, err
		}
		if done {
			if service.verbosity >= 4 {
				fmt.Println("capture is done, returning")
			}
			aduValue, err := service.driver.GetADUValue()
			if err != nil {
				fmt.Println("TheSkyServiceInstance/CaptureAndMeasureFlatFrame error from GetADUValue:", err)
				return 0, err
			}
			if service.simulateFlatCapture {
				simulatedAduValue, _ := service.simulatedFrameCapture(exposure, binning, filterSlot, downloadTime, saveImage)
				if service.verbosity >= 4 {
					fmt.Printf("Simulating ADU value, overrode %d with %d", aduValue, simulatedAduValue)
				}
				aduValue = simulatedAduValue
			}
			if service.verbosity >= 4 {
				fmt.Println("   Returned ADU value:", aduValue)
			}
			return aduValue, nil
		}
		if secondsWaitedSoFar > maximumWaitSeconds {
			return 0, errors.New("TheSkyServiceInstance/CaptureAndMeasureFlatFrame: Timeout waiting for capture to finish")
		}
		if service.verbosity >= 4 {
			fmt.Println("Camera not finished. Delaying ", pollingInterval)
		}
		if _, err := service.delayService.DelayDuration(int(math.Round(pollingInterval))); err != nil {
			fmt.Println("TheSkyServiceInstance/CaptureDarkFrame error from polling delaypkg service:", err)
			return 0, err
		}
		secondsWaitedSoFar += pollingInterval
	}
}

// Simulate a frame capture by doing a simple linear formula with experimental slope and intercept,
// and add a bit of noise
func (service *TheSkyServiceInstance) simulatedFrameCapture(exposure float64, binning int, filterSlot int, _ float64, _ bool) (int64, error) {
	var slope float64
	var intercept float64
	if (binning == 1) && (filterSlot == 4) {
		// Luminance, binned 1x1
		slope = 721.8
		intercept = 19817.0
	} else if (binning == 2) && (filterSlot == 1) {
		// Red filter, binned 2x2
		slope = 7336.7
		intercept = -100.48
	} else if (binning == 2) && (filterSlot == 2) {
		// Green filter, binned 2x2
		slope = 11678.0
		intercept = -293.09
	} else if (binning == 2) && (filterSlot == 3) {
		// Blue filter, binned 2x2
		slope = 6820.4
		intercept = 1858.3
	} else if (binning == 1) && (filterSlot == 5) {
		// H-alpha filter, binned 1x1
		slope = 67.247
		intercept = 2632.7
	} else {
		slope = 721.8
		intercept = 19817.0
	}
	calculatedResult := slope*exposure + intercept

	// Now we'll put a small percentage noise into the value, so it has some variability for realism
	randFactorZeroCentered := service.simulationNoiseFraction * (rand.Float64() - 0.5)
	noisyResult := calculatedResult + randFactorZeroCentered*calculatedResult
	roundedNoisyResult := math.Round(noisyResult)
	intResult := int64(math.Min(roundedNoisyResult, 65535.0))

	if service.verbosity >= 4 {
		fmt.Printf("Simulated flat adu for exp %g, binning %d, filter %d = %d\n", exposure, binning, filterSlot, intResult)
	}
	return intResult, nil
}

// HasFilterWheel determines whether the camera has a filter wheel.
//
//	 There is no API call to determine this, so we will infer it this way:
//	Determine if the filter wheel is connected
//		If yes, then there is a filter wheel (duh)
//		If no, then try to connect.
//			If that fails, there is no filter wheel.
//			If the connect succeeds, then there is a filter wheel; and disconnect again
func (service *TheSkyServiceInstance) HasFilterWheel() (bool, error) {
	if service.verbosity >= 4 || service.debug {
		fmt.Println("TheSkyServiceInstance/HasFilterWheel ")
	}

	// Ask if filter wheel is connected
	isConnected, err := service.driver.FilterWheelIsConnected()
	//	Success means there is a wheel
	if err != nil {
		fmt.Println("HasFilterWheel error from driver checking if connected:", err)
		return false, err
	}
	if isConnected {
		return true, nil
	}

	// Not connected.  Try to connect
	err = service.driver.FilterWheelConnect()

	//	Failure?  No filter wheel
	if err != nil {
		//fmt.Println("Filterwheel error connect code: ", err)
		return false, nil
	}
	//	Success?  Filter wheel.  And disconnect.

	_ = service.driver.FilterWheelDisconnect()
	return true, nil

}

// NumberOfFilters returns the number of filters defined for the filter wheel.
//
//	Although the server provides a function for this, we are not using it because the filter wheel simulator
//	returns an absurd number of filters with names that get filled in automatically.
//	Instead, we are going to retrieve the actual filter names, and count up to, not including, the first blank one
func (service *TheSkyServiceInstance) NumberOfFilters() (int, error) {
	if service.verbosity >= 4 || service.debug {
		fmt.Println("TheSkyServiceInstance/NumberOfFilters ")
	}

	// Ask driver for filter names
	filterNames, err := service.driver.FilterNames()
	if err != nil {
		fmt.Println("NumberOfFilters error from driver retrieving filter names:", err)
		return 0, err
	}
	count := 0
	for i := 0; i < len(filterNames); i++ {
		name := filterNames[i]
		if strings.TrimSpace(name) == "" {
			break
		}
		count++
	}
	return count, nil
}

func (service *TheSkyServiceInstance) FilterNames() ([]string, error) {
	if service.verbosity >= 4 || service.debug {
		fmt.Println("TheSkyServiceInstance/FilterNames ")
	}

	// Ask driver for filter names
	filterNames, err := service.driver.FilterNames()
	if err != nil {
		fmt.Println("FilterNames error from driver retrieving filter names:", err)
		return []string{}, err
	}
	count := 0
	for i := 0; i < len(filterNames); i++ {
		nameTrimmed := strings.ToLower(strings.TrimSpace(filterNames[i]))
		if nameTrimmed == "" {
			break
		}
		filterNames[i] = nameTrimmed
		count++
	}
	return filterNames[:count], nil

}

//func (service *TheSkyServiceInstance) xxxxxx(args and types) (return, error) {
