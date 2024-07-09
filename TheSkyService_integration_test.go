package goTheSkyX

import (
	"github.com/RMcDOttawa/goMockableDelay"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"strings"
	"sync"
	"testing"
)

// The following are INTEGRATION tests - rather than using mocks, it actually calls the real TheSkyService
// and TheSkyDriver services.  It is used to test the integration of the services with the real TheSkyX.
// It is not used in the normal build process, but is useful for testing the real services.
// To run these tests, you need to have TheSkyX running on the same machine as the tests are running.
// TheSkyX must be running in Server mode, and the port number must match the port number in the tests.
// TheSkyX must be connected to a camera simulator (not real camera - so response is immediate) and filter wheel simulator.

//	The following constant turns the tests off to prevent them from running during continuous integration
//	or when test ./... is used, except when we want to run them.

const runIntegrationTests = true

// Since these tests are interacting with the real server, we need to use a mutext to ensure they are serialized
var testFuncMutex sync.Mutex

const cameraWaitPollingSeconds = 1
const cameraWaitTimeoutMinutes = 2

const isThereInFactAFilterWheel = true
const expectedNumberOfFilters = 5 // Number of slots in the bisque filter wheel simulator
var expectedFilterNames = [...]string{"red", "green", "blue", "luminance", "ha"}

func TestServerIntegration(t *testing.T) {
	if runIntegrationTests {
		t.Run("Integration test camera cooler", func(t *testing.T) {
			testFuncMutex.Lock()
			defer testFuncMutex.Unlock()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			const targetTemperature = -10.0

			mockDelayService := goMockableDelay.NewMockDelayService(ctrl)
			realDelayService := goMockableDelay.NewDelayService(false, 1)
			server := NewTheSkyService(mockDelayService, false, 1, true)

			err := server.Connect("localhost", 3040)
			require.Nil(t, err, "Unable to connect to service")
			err = server.ConnectCamera()
			require.Nil(t, err, "Unable to connect to camera")
			err = server.WaitForCameraInactive(cameraWaitPollingSeconds, cameraWaitTimeoutMinutes)
			require.Nil(t, err, "Camera did not become inactive from previous test")
			err = server.StartCooling(targetTemperature)
			require.Nil(t, err, "Unable to start camera cooling")
			// First temperature poll is sometimes nonsense, so discard it
			_, _ = realDelayService.DelayDuration(1)
			temperature, err := server.GetCameraTemperature()
			require.Nil(t, err, "Error on first temperature poll")
			_, _ = realDelayService.DelayDuration(2)
			temperature, err = server.GetCameraTemperature()
			require.Nil(t, err, "Error on second temperature poll")
			require.Equal(t, targetTemperature, temperature, "Simulated camera temperature not cooled to target")
			err = server.StopCooling()
			require.Nil(t, err, "Unable to turn off cooling")
			err = server.Close()
			require.Nil(t, err, "Unable to close server")
		})

		t.Run("Integration test measuring download time", func(t *testing.T) {
			testFuncMutex.Lock()
			defer testFuncMutex.Unlock()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			realDelayService := goMockableDelay.NewDelayService(false, 1)
			server := NewTheSkyService(realDelayService, false, 1, true)

			err := server.Connect("localhost", 3040)
			require.Nil(t, err, "Unable to connect to service")
			err = server.ConnectCamera()
			require.Nil(t, err, "Unable to connect to camera")
			err = server.WaitForCameraInactive(cameraWaitPollingSeconds, cameraWaitTimeoutMinutes)
			require.Nil(t, err, "Camera did not become inactive from previous test")

			const arbitraryBinning = 1
			downloadTime, err := server.MeasureDownloadTime(arbitraryBinning)
			require.Nil(t, err, "Unable to measure download time")
			require.Greater(t, downloadTime, 0.0, "Download time is zero")

			err = server.Close()
			require.Nil(t, err, "Unable to close server")
		})

		t.Run("Integration test dark frame capture", func(t *testing.T) {
			testFuncMutex.Lock()
			defer testFuncMutex.Unlock()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			realDelayService := goMockableDelay.NewDelayService(false, 1)
			server := NewTheSkyService(realDelayService, false, 1, true)

			err := server.Connect("localhost", 3040)
			require.Nil(t, err, "Unable to connect to service")
			err = server.ConnectCamera()
			require.Nil(t, err, "Unable to connect to camera")
			err = server.WaitForCameraInactive(cameraWaitPollingSeconds, cameraWaitTimeoutMinutes)
			require.Nil(t, err, "Camera did not become inactive from previous test")

			const arbitraryBinning = 1
			const arbitraryExposureLength = 2.0
			downloadTime, err := server.MeasureDownloadTime(arbitraryBinning)
			require.Nil(t, err, "Unable to measure download time")

			err = server.CaptureDarkFrame(arbitraryBinning, arbitraryExposureLength, downloadTime)
			require.Nil(t, err, "Unable to capture dark frame")

			err = server.Close()
			require.Nil(t, err, "Unable to close server")

		})

		t.Run("Integration test bias frame capture", func(t *testing.T) {
			testFuncMutex.Lock()
			defer testFuncMutex.Unlock()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			realDelayService := goMockableDelay.NewDelayService(false, 1)
			server := NewTheSkyService(realDelayService, false, 1, true)

			err := server.Connect("localhost", 3040)
			require.Nil(t, err, "Unable to connect to service")
			err = server.ConnectCamera()
			require.Nil(t, err, "Unable to connect to camera")
			err = server.WaitForCameraInactive(cameraWaitPollingSeconds, cameraWaitTimeoutMinutes)
			require.Nil(t, err, "Camera did not become inactive from previous test")

			const arbitraryBinning = 1
			downloadTime, err := server.MeasureDownloadTime(arbitraryBinning)
			require.Nil(t, err, "Unable to measure download time")

			err = server.CaptureBiasFrame(arbitraryBinning, downloadTime)
			require.Nil(t, err, "Unable to capture bias frame")

			err = server.Close()
			require.Nil(t, err, "Unable to close server")

		})

		t.Run("Integration test flat frame capture", func(t *testing.T) {
			testFuncMutex.Lock()
			defer testFuncMutex.Unlock()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			realDelayService := goMockableDelay.NewDelayService(false, 1)
			server := NewTheSkyService(realDelayService, false, 1, true)

			err := server.Connect("localhost", 3040)
			require.Nil(t, err, "Unable to connect to service")
			err = server.ConnectCamera()
			require.Nil(t, err, "Unable to connect to camera")
			err = server.WaitForCameraInactive(cameraWaitPollingSeconds, cameraWaitTimeoutMinutes)
			require.Nil(t, err, "Camera did not become inactive from previous test")

			const arbitraryBinning = 1
			const arbitraryExposureLength = 2.0
			const arbitraryFilterSlot = 1
			const dontSave = false
			downloadTime, err := server.MeasureDownloadTime(arbitraryBinning)
			require.Nil(t, err, "Unable to measure download time")

			adus, err := server.CaptureAndMeasureFlatFrame(arbitraryExposureLength, arbitraryBinning, arbitraryFilterSlot, downloadTime, dontSave)
			require.Nil(t, err, "Unable to capture flat frame")
			require.Greater(t, adus, int64(0))

			err = server.Close()
			require.Nil(t, err, "Unable to close server")

		})

		t.Run("Integration test filter wheel functions", func(t *testing.T) {
			t.Run("Detect filter wheel", func(t *testing.T) {
				testFuncMutex.Lock()
				defer testFuncMutex.Unlock()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockDelayService := goMockableDelay.NewMockDelayService(ctrl)
				server := NewTheSkyService(mockDelayService, false, 1, true)
				err := server.Connect("localhost", 3040)
				require.Nil(t, err, "Unable to connect to service")
				err = server.ConnectCamera()
				require.Nil(t, err, "Unable to connect to camera")
				err = server.WaitForCameraInactive(cameraWaitPollingSeconds, cameraWaitTimeoutMinutes)
				require.Nil(t, err, "Camera did not become inactive from previous test")

				hasFilter, err := server.HasFilterWheel()
				require.Nil(t, err, "Unable to check filter wheel")
				require.Equal(t, isThereInFactAFilterWheel, hasFilter, "Incorrect filter wheel detection")
			})

			//	Number of filters
			t.Run("Report number of slots on filter wheel", func(t *testing.T) {
				testFuncMutex.Lock()
				defer testFuncMutex.Unlock()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockDelayService := goMockableDelay.NewMockDelayService(ctrl)
				server := NewTheSkyService(mockDelayService, false, 1, true)
				err := server.Connect("localhost", 3040)
				require.Nil(t, err, "Unable to connect to service")
				err = server.ConnectCamera()
				require.Nil(t, err, "Unable to connect to camera")
				err = server.WaitForCameraInactive(cameraWaitPollingSeconds, cameraWaitTimeoutMinutes)
				require.Nil(t, err, "Camera did not become inactive from previous test")

				numberOfFilters, err := server.NumberOfFilters()
				require.Nil(t, err, "Unable to number of filter slots from filter wheel")
				require.Equal(t, expectedNumberOfFilters, numberOfFilters, "Incorrect filter wheel slot count")
			})

			//	Array of filter names
			t.Run("Retrieve array of names of filter wheel slots", func(t *testing.T) {
				testFuncMutex.Lock()
				defer testFuncMutex.Unlock()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockDelayService := goMockableDelay.NewMockDelayService(ctrl)
				server := NewTheSkyService(mockDelayService, false, 1, true)
				err := server.Connect("localhost", 3040)
				require.Nil(t, err, "Unable to connect to service")
				err = server.ConnectCamera()
				require.Nil(t, err, "Unable to connect to camera")
				err = server.WaitForCameraInactive(cameraWaitPollingSeconds, cameraWaitTimeoutMinutes)
				require.Nil(t, err, "Camera did not become inactive from previous test")

				numberOfFilters, err := server.NumberOfFilters()
				require.Nil(t, err, "Unable to number of filter slots from filter wheel")

				filterNames, err := server.FilterNames()
				require.Nil(t, err, "Unable to retrieve filter wheel slot names")
				require.Equal(t, numberOfFilters, len(filterNames), "Wrong number of filter names returned")
				for i := 0; i < len(filterNames); i++ {
					require.Equal(t, strings.ToLower(expectedFilterNames[i]),
						strings.ToLower(filterNames[i]), "Expected filter name not returned")
				}
			})

		})

		//t.Run("Integration test xxx", func(t *testing.T) {
		//	testFuncMutex.Lock()
		//	defer testFuncMutex.Unlock()
		//
		//	ctrl := gomock.NewController(t)
		//	defer ctrl.Finish()
		//
		//	mockDelayService := goMockableDelay.NewMockDelayService(ctrl)
		//	server := NewTheSkyService(mockDelayService, false, 1)
		//
		//})
	}

}
