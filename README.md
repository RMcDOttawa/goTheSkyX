# goTheSkyX

This module is an interface service to interact with Software Bisque's TheSkyX Pro application, for purposes of controlling an attached camera, to take calibration frames. There is also some limited ability to control the telescope mount - only minimal services to assist with calibration frames (for example, slewing the scope to point at a flat frame panel, or dithering flat calibration frames).

- TheSkyService is the high-level service that an application should use.
- TheSkyDriver is a low-level set of services used by TheSkyService.  Users are not normally expected to interact directly with TheSkyDriver.  
- When creating a TheSkyService instance, you must pass in a MockableDelay service object (see separate module "[github.com/RMcDOttawa/goMockableDelay](https://github.com/RMcDOttawa/goMockableDelay)" for that). This allows you to pass in a relay delay service for real use, or a mocked delay service for testing.

TheSkyService functions

| Function             | Arguments                                      | Purpose                                                                                                                                                                                                                                                                           |
| -------------------- | ---------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| NewTheSkyService     | delayService, debug, verbosity                 | Creates a new delay service object, returning a pointer.                                                                                                                                                                                                                          |
| SetDebug             | boolean                                        | Sets the "debug" flag for the service                                                                                                                                                                                                                                             |
| SetVerbosity         | int                                            | Sets the verbosity level, from 0 to 5                                                                                                                                                                                                                                             |
| Connect              | server string, port int                        | Connect to the service, giving it the address and port number of TheSkyX running somewhere on your network                                                                                                                                                                        |
| Close                |                                                | Close the server connection                                                                                                                                                                                                                                                       |
| ConnectCamera        |                                                | Ask TheSkyX to connect to the camera                                                                                                                                                                                                                                              |
| StartCooling         | temperature float                              | Ask the camera to switch on its cooler and begin cooling to the given target temperature                                                                                                                                                                                          |
| StopCooling          |                                                | Ask the camera to switch off its cooler                                                                                                                                                                                                                                           |
| GetCameraTemperature |                                                | Retrieve the current camera temperature                                                                                                                                                                                                                                           |
| MeasureDownloadTime  |                                                | Measure how long it takes the camera to download an image of the given binning level (return seconds as a float number). The intent is that you would do this once before taking a large number of dark, bias, or flat frames, passing the download time to the capture function. |
| CaptureDarkFrame     | binning int, seconds float, downloadtime float | Take a dark frame of the given binning and exposure length. Provide the measured download time to assist the service in knowing how long to wait.  Note that the frame itself is not returned - the file is stored, by TheSkyX, whereever its AutoSave setting has files going.   |
| CaptureBiasFrame     | binning int, downloadtime float                | Take a bias frame of the given binning . Provide the measured download time to assist the service in knowing how long to wait. Note that the frame itself is not returned - the file is stored, by TheSkyX, whereever its AutoSave setting has files going.                       |

Create and use a MockTheSkyService using the normal mocking framework and inject it into your code under test for testing purposes.

e.g.,

````
mockTheSkyService := goMockableDelay.NewMockTheSkyService(ctrl)
mockTheSkyService.EXPECT().Connect("localhost",3040).Return(nil)
mockTheSkyService.EXPECT().Close().Return(nil)
mockTheSkyService.EXPECT().ConnectCamera().Return(nil)
mockTheSkyService.EXPECT().GetCameraTemperature().Return(-10.0, nil)
````