package goTheSkyX

import (
	"errors"
	"github.com/RMcDOttawa/goMockableDelay"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

// TestDarkCapture tests the ability to capture a single dark frame.
// We mock the TheSkyDriver service to simulate responses from the driver
func TestDarkCapture(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Test of an ideal acquisition - we start the camera acquisition, wait a calculated amount
	// of time, and then see that the camera reports done
	t.Run("capture dark frame ready on time", func(t *testing.T) {
		//mockDelayService, service, mockDriver := setUpDarkCaptureTest(ctrl)
		mockDelayService := goMockableDelay.NewMockDelayService(ctrl)
		service := NewTheSkyService(mockDelayService, false, 0)
		// Plug mock driver into service
		mockDriver := NewMockTheSkyDriver(ctrl)
		service.SetDriver(mockDriver)

		const binning = 1
		const seconds = 20.0
		const downloadTime = 5.0
		mockDriver.EXPECT().StartDarkFrameCapture(binning, seconds, downloadTime).Return(nil)
		//	Initial delaypkg while waiting for exposure
		initialDelay := int(math.Round(seconds + downloadTime + AndALittleExtra)) // from service
		mockDelayService.EXPECT().DelayDuration(initialDelay).Return(initialDelay, nil)
		//	Report capture done on first check
		mockDriver.EXPECT().IsCaptureDone().Return(true, nil)

		err := service.CaptureDarkFrame(binning, seconds, downloadTime)

		require.Nil(t, err, "CaptureDarkFrame failed")
	})

	//Test of an acquisition requiring extra wait.  We start the camera acquisition, wait a calculated amount,
	//then find it isn't finished. So we loop and poll two more times, then it is done.
	t.Run("capture dark frame requiring two extra waits", func(t *testing.T) {
		//mockDelayService, service, mockDriver := setUpDarkCaptureTest(ctrl)
		mockDelayService := goMockableDelay.NewMockDelayService(ctrl)
		service := NewTheSkyService(mockDelayService, false, 0)
		// Plug mock driver into service
		mockDriver := NewMockTheSkyDriver(ctrl)
		service.SetDriver(mockDriver)

		const binning = 1
		const seconds = 20.0
		const downloadTime = 5.0
		//	The mock driver will be asked to initiate capture, and this will report success
		mockDriver.EXPECT().StartDarkFrameCapture(1, seconds, downloadTime).Return(nil)
		//	Mock the initial delaypkg while waiting for exposure
		initialDelay := int(math.Round(seconds + downloadTime + AndALittleExtra)) // from service
		mockDelayService.EXPECT().DelayDuration(initialDelay).Return(initialDelay, nil)
		//	Mock extra waits between polls
		mockDelayService.EXPECT().DelayDuration(2).Return(1, nil).Times(2)
		//	Mock camera status to report capture not done on first or second check; done on third
		mockDriver.EXPECT().IsCaptureDone().Return(false, nil)
		mockDriver.EXPECT().IsCaptureDone().Return(false, nil)
		mockDriver.EXPECT().IsCaptureDone().Return(true, nil)

		err := service.CaptureDarkFrame(binning, seconds, downloadTime)
		require.Nil(t, err, "CaptureDarkFrame failed")
	})

	// Test of an acquisition timing out.  we start the camera acquisition, wait a calculated amount,
	// then continue to wait and poll, only to eventually time out with no completion.
	t.Run("capture dark frame times out while waiting", func(t *testing.T) {
		//mockDelayService, service, mockDriver := setUpDarkCaptureTest(ctrl)

		mockDelayService := goMockableDelay.NewMockDelayService(ctrl)
		service := NewTheSkyService(mockDelayService, false, 0)
		// Plug mock driver into service
		mockDriver := NewMockTheSkyDriver(ctrl)
		service.SetDriver(mockDriver)

		const binning = 1
		const seconds = 20.0
		const downloadTime = 5.0
		//	The mock driver will be asked to initiate capture, and this will report success
		mockDriver.EXPECT().StartDarkFrameCapture(1, seconds, downloadTime).Return(nil)
		//	Initial delay while waiting for exposure
		initialDelay := int(math.Round(seconds + downloadTime + AndALittleExtra)) // from service
		mockDelayService.EXPECT().DelayDuration(initialDelay).Return(initialDelay, nil)
		//	Extra waits between polls
		mockDelayService.EXPECT().DelayDuration(2).AnyTimes().Return(1, nil)
		//	Report capture not done no matter how often we ask, so the logic will eventually time out
		mockDriver.EXPECT().IsCaptureDone().AnyTimes().Return(false, nil)

		err := service.CaptureDarkFrame(binning, seconds, downloadTime)
		require.NotNil(t, err, "CaptureDarkFrame should have timed out")
		require.ErrorContains(t, err, "Timeout waiting for capture")
	})

}

func TestFlatCapture(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Test of an ideal acquisition - we start the camera acquisition, wait a calculated amount
	// of time, and then see that the camera reports done, and returns correct ADU setting
	t.Run("capture flat frame ready on time", func(t *testing.T) {
		//mockDelayService, service, mockDriver := setUpDarkCaptureTest(ctrl)
		mockDelayService := goMockableDelay.NewMockDelayService(ctrl)
		service := NewTheSkyService(mockDelayService, false, 0)
		service.SetSimulateFlatCapture(false)
		// Plug mock driver into service
		mockDriver := NewMockTheSkyDriver(ctrl)
		service.SetDriver(mockDriver)

		const binning = 1
		const seconds = 14.0
		const downloadTime = 5.0
		const saveImageFlag = false
		const filterSlot = 1
		const arbitraryAduValue = int64(30000)
		mockDriver.EXPECT().StartFlatFrameCapture(binning, seconds, filterSlot, downloadTime, saveImageFlag).Return(nil)
		//	Initial delaypkg while waiting for exposure
		initialDelay := int(math.Round(seconds + downloadTime + AndALittleExtra)) // from service
		mockDelayService.EXPECT().DelayDuration(initialDelay).Return(initialDelay, nil)
		//	Report capture done on first check
		//	Mock camera status to report capture not done on first or second check; done on third
		mockDriver.EXPECT().IsCaptureDone().Return(true, nil)
		mockDriver.EXPECT().GetADUValue().Return(arbitraryAduValue, nil)

		aduValue, err := service.CaptureAndMeasureFlatFrame(seconds, binning, filterSlot, downloadTime, saveImageFlag)
		require.Nil(t, err, "CaptureDarkFrame failed")
		require.Equal(t, arbitraryAduValue, aduValue, "Expected to get 30000")
	})

	//Test of an acquisition requiring extra wait.  We start the camera acquisition, wait a calculated amount,
	//then find it isn't finished. So we loop and poll two more times, then it is done.
	t.Run("capture flat frame requiring two extra waits", func(t *testing.T) {
		//mockDelayService, service, mockDriver := setUpDarkCaptureTest(ctrl)
		mockDelayService := goMockableDelay.NewMockDelayService(ctrl)
		service := NewTheSkyService(mockDelayService, false, 0)
		service.SetSimulateFlatCapture(false)
		// Plug mock driver into service
		mockDriver := NewMockTheSkyDriver(ctrl)
		service.SetDriver(mockDriver)

		const binning = 1
		const seconds = 14.0
		const downloadTime = 5.0
		const saveImageFlag = false
		const arbitraryAduValue = int64(30000)
		const filterSlot = 1
		//	The mock driver will be asked to initiate capture, and this will report success
		mockDriver.EXPECT().StartFlatFrameCapture(binning, seconds, filterSlot, downloadTime, saveImageFlag).Return(nil)
		//	Mock the initial delay pkg while waiting for exposure
		initialDelay := int(math.Round(seconds + downloadTime + AndALittleExtra)) // from service
		mockDelayService.EXPECT().DelayDuration(initialDelay).Return(initialDelay, nil)
		//	Mock extra waits between polls
		mockDelayService.EXPECT().DelayDuration(2).AnyTimes().Return(1, nil)
		//	Mock camera status to report capture not done on first or second check; done on third
		mockDriver.EXPECT().IsCaptureDone().Return(false, nil)
		mockDriver.EXPECT().IsCaptureDone().Return(false, nil)
		mockDriver.EXPECT().IsCaptureDone().Return(true, nil)
		mockDriver.EXPECT().GetADUValue().Return(arbitraryAduValue, nil)

		aduValue, err := service.CaptureAndMeasureFlatFrame(seconds, binning, filterSlot, downloadTime, saveImageFlag)
		require.Nil(t, err, "CaptureDarkFrame failed")
		require.Equal(t, arbitraryAduValue, aduValue, "Expected to get 30000")
	})
	//
	// Test of an acquisition timing out.  we start the camera acquisition, wait a calculated amount,
	// then continue to wait and poll, only to eventually time out with no completion.
	t.Run("capture flat frame timing out without finishing", func(t *testing.T) {
		//mockDelayService, service, mockDriver := setUpDarkCaptureTest(ctrl)
		mockDelayService := goMockableDelay.NewMockDelayService(ctrl)
		service := NewTheSkyService(mockDelayService, false, 0)
		service.SetSimulateFlatCapture(false)
		// Plug mock driver into service
		mockDriver := NewMockTheSkyDriver(ctrl)
		service.SetDriver(mockDriver)

		const binning = 1
		const seconds = 14.0
		const downloadTime = 5.0
		const saveImageFlag = false
		const arbitraryAduValue = int64(30000)
		const filterSlot = 1
		//	The mock driver will be asked to initiate capture, and this will report success
		mockDriver.EXPECT().StartFlatFrameCapture(binning, seconds, filterSlot, downloadTime, saveImageFlag).Return(nil)
		//	Mock the initial delay pkg while waiting for exposure
		initialDelay := int(math.Round(seconds + downloadTime + AndALittleExtra)) // from service
		mockDelayService.EXPECT().DelayDuration(initialDelay).Return(initialDelay, nil)
		//	Mock extra waits between polls
		mockDelayService.EXPECT().DelayDuration(2).Return(1, nil).Times(2)
		mockDelayService.EXPECT().DelayDuration(2).AnyTimes().Return(1, nil)
		//	Mock camera status to report capture not done on first or second check; done on third
		mockDriver.EXPECT().IsCaptureDone().AnyTimes().Return(false, nil)
		mockDriver.EXPECT().GetADUValue().AnyTimes().Return(arbitraryAduValue, nil)

		_, err := service.CaptureAndMeasureFlatFrame(seconds, binning, filterSlot, downloadTime, saveImageFlag)
		require.NotNil(t, err, "capture flat should have failed")
		require.ErrorContains(t, err, "Timeout waiting for capture to finish")
	})

}

func TestFilterWheel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Test ability to detect filter wheel when present and already connected
	t.Run("detect filter wheel", func(t *testing.T) {
		mockDelayService := goMockableDelay.NewMockDelayService(ctrl)
		service := NewTheSkyService(mockDelayService, false, 0)
		// Plug mock driver into service
		mockDriver := NewMockTheSkyDriver(ctrl)
		service.SetDriver(mockDriver)

		mockDriver.EXPECT().FilterWheelIsConnected().Return(true, nil)

		hasFilterWheel, err := service.HasFilterWheel()
		require.Nil(t, err, "Unable to query filter wheel")
		require.True(t, hasFilterWheel, "Expected filter wheel")

	})

	// Test ability to detect filter wheel when present but not yet  connected
	t.Run("detect filter wheel when not connected", func(t *testing.T) {
		mockDelayService := goMockableDelay.NewMockDelayService(ctrl)
		service := NewTheSkyService(mockDelayService, false, 0)
		// Plug mock driver into service
		mockDriver := NewMockTheSkyDriver(ctrl)
		service.SetDriver(mockDriver)

		mockDriver.EXPECT().FilterWheelIsConnected().Return(false, nil)
		mockDriver.EXPECT().FilterWheelConnect().Return(nil)
		mockDriver.EXPECT().FilterWheelDisconnect().Return(nil)

		hasFilterWheel, err := service.HasFilterWheel()
		require.Nil(t, err, "Unable to query filter wheel")
		require.True(t, hasFilterWheel, "Expected filter wheel")

	})

	// Test ability to detect absence of a filter wheel when none is present
	t.Run("detect filter wheel", func(t *testing.T) {
		mockDelayService := goMockableDelay.NewMockDelayService(ctrl)
		service := NewTheSkyService(mockDelayService, false, 0)
		// Plug mock driver into service
		mockDriver := NewMockTheSkyDriver(ctrl)
		service.SetDriver(mockDriver)

		mockDriver.EXPECT().FilterWheelIsConnected().Return(false, nil)
		mockDriver.EXPECT().FilterWheelConnect().Return(errors.New("can't connect"))

		hasFilterWheel, err := service.HasFilterWheel()
		require.Nil(t, err, "Unable to query filter wheel")
		require.False(t, hasFilterWheel, "Expected to determine there is no filter wheel")

	})

}
