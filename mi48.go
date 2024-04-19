package mi48

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"io"
	"log"
	"math"
	"slices"
	"strings"
	"sync"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

type MI48 struct {
	serialPort      serial.Port
	CameraModel     string
	SerialInfo      string
	FirmwareVersion string
	MaxFPS          float64
	cameraType      uint8

	readMutex sync.Mutex
}

type medianFilterMode uint8

var MEDIAN_DISABLED = medianFilterMode(0)
var MEDIAN_KERNEL_SIZE_3 = medianFilterMode(3)
var MEDIAN_KERNEL_SIZE_5 = medianFilterMode(5)

type FilterSettings struct {
	Temporal       uint16
	RollingAverage uint8
	Median         medianFilterMode
}

var DefaultFilterSettings = FilterSettings{Temporal: 125, RollingAverage: 4, Median: MEDIAN_DISABLED}

type NETDConfig struct {
	Enabled    bool
	RowInFrame bool
	Factor     uint8
	X          uint8
	Y          uint8
}

var DefaultNETDConfig = NETDConfig{Enabled: false, RowInFrame: false, Factor: 0x14, X: 0, Y: 0}

func Open(serialPort ...string) (*MI48, error) {
	portName := ""
	var err error

	if len(serialPort) == 0 {
		// Try to autodetect the serial port MI48 is connected to
		portName, err = getSerialPort()
		if err != nil {
			return nil, fmt.Errorf("failed to Open MI48 device: %w", err)
		}
	} else {
		portName = serialPort[0]
	}

	p, err := serial.Open(portName, &serial.Mode{}) // We don't care about baudrate etc. this is a USB-CDC device
	if err != nil {
		return nil, fmt.Errorf("failed to Open MI48 device: %w", err)
	}

	camera := MI48{serialPort: p}

	err = camera.init()

	return &camera, err
}

func (m *MI48) init() (err error) {
	m.cameraType, err = m.getCameraType()
	if err != nil {
		return
	}

	var ok bool
	if m.CameraModel, ok = cameraTypes[m.cameraType]; !ok {
		return fmt.Errorf("unknown camera type: %d", m.cameraType)
	}

	m.SerialInfo, err = m.getCameraID()
	if err != nil {
		return
	}

	m.FirmwareVersion, err = m.getFirmwareVersion()
	if err != nil {
		return
	}

	m.MaxFPS, err = m.getMaxFPS()
	if err != nil {
		return
	}

	err = m.SetFilters(DefaultFilterSettings)
	if err != nil {
		return
	}

	if err := m.SetTemperatureOffset(0.0); err != nil {
		return err
	}

	if err := m.SetNETD(DefaultNETDConfig); err != nil {
		return err
	}

	return
}

func (m *MI48) getMaxFPS() (float64, error) {
	cameraType, err := m.readRegister(SENXOR_TYPE)
	if err != nil {
		return 0, fmt.Errorf("failed to get camera type: %w", err)
	}

	switch cameraType[0] {
	case 0:
		fallthrough
	case 1:
		return 25.5, nil
	case 2:
		return 28.57, nil
	default:
		// We don't know
		return 0.0, nil
	}
}

func (m *MI48) SetTemperatureOffset(offset float64) error {
	offsetValue := offset / 0.05
	if offsetValue < 0 {
		return m.writeRegister(OFFSET_CORR, uint8(256-math.Abs(offsetValue)))
	} else {
		return m.writeRegister(OFFSET_CORR, uint8(offsetValue))
	}
}

func (m *MI48) SetFramerate(frameRate float64) (actualFrameRate float64, err error) {
	if frameRate == 0 || frameRate > m.MaxFPS {
		return 0.0, fmt.Errorf("invalid target frame rate %f, must be 0 < target < MaxFPS", frameRate)
	}
	optimalDivisor := m.MaxFPS / frameRate
	divisor := int(math.Round(optimalDivisor))

	err = m.writeRegister(FRAME_RATE, uint8(divisor))

	return m.MaxFPS / float64(divisor), err
}

func (m *MI48) GetFramerate() (float64, error) {
	divisorData, err := m.readRegister(FRAME_RATE)
	if err != nil {
		return 0.0, fmt.Errorf("failed to read frame rate: %w", err)
	}
	divisor := divisorData[0]
	if divisor == 0 {
		return 0.0, fmt.Errorf("invalid frame rate divisor: 0")
	}
	return m.MaxFPS / float64(divisor), nil
}

func (m *MI48) SetFilters(filterSettings FilterSettings) error {
	fltCtrl := uint8(0)
	fltSet1Lsb := uint8(0)
	fltSet1Msb := uint8(0)
	fltSet2 := uint8(0)

	if filterSettings.Temporal > 0 {
		fltCtrl |= 0x03
		fltSet1Lsb = uint8(filterSettings.Temporal & 0xFF)
		fltSet1Msb = uint8(filterSettings.Temporal & 0xFF00 >> 8)
	}
	if filterSettings.RollingAverage > 0 {
		fltCtrl |= 0x04
		fltSet2 = filterSettings.RollingAverage
	}
	if filterSettings.Median > 0 {
		fltCtrl |= 0x40
		if filterSettings.Median > 3 {
			fltCtrl |= 0x20
		}
	}

	err := m.writeRegister(FILTER_SETTING_1_LSB, fltSet1Lsb)
	if err != nil {
		return fmt.Errorf("failed to set filter settings: %w", err)
	}
	err = m.writeRegister(FILTER_SETTING_1_MSB, fltSet1Msb)
	if err != nil {
		return fmt.Errorf("failed to set filter settings: %w", err)
	}
	err = m.writeRegister(FILTER_SETTING_2, fltSet2)
	if err != nil {
		return fmt.Errorf("failed to set filter settings: %w", err)
	}
	err = m.writeRegister(FILTER_CONTROL, fltCtrl)
	if err != nil {
		return fmt.Errorf("failed to set filter control: %w", err)
	}

	return nil
}

func (m *MI48) SetNETD(netdConfig NETDConfig) error {
	if err := m.writeRegister(NETD_FACTOR, netdConfig.Factor); err != nil {
		return fmt.Errorf("failed to set NETD factor: %w", err)
	}
	if err := m.writeRegister(NETD_PIXEL_Y, netdConfig.Y); err != nil {
		return fmt.Errorf("failed to set NETD pixel Y: %w", err)
	}
	if err := m.writeRegister(NETD_PIXEL_X, netdConfig.X); err != nil {
		return fmt.Errorf("failed to set NETD pixel X: %w", err)
	}
	netd_config := uint8(0x00)
	if netdConfig.Enabled {
		netd_config |= 0x01
		if netdConfig.RowInFrame {
			netd_config |= 0x02
		}
	}

	if err := m.writeRegister(NETD_CONFIG, netd_config); err != nil {
		return fmt.Errorf("failed to set NETD config: %w", err)
	}

	return nil
}

func (m *MI48) StartStream() (context.CancelFunc, <-chan *image.Gray16, error) {
	if err := m.writeRegister(FRAME_MODE, uint8(CONTNIUOUS_STREAM)); err != nil {
		return nil, nil, fmt.Errorf("failed to start stream: %w", err)
	}
	streamContext, cancel := context.WithCancel(context.Background())

	// Create a channel that transports 16bit images
	videoStream := make(chan *image.Gray16, 10)
	go func() {
		defer close(videoStream)

		for {
			select {
			case <-streamContext.Done():
				return
			default:
				m.readMutex.Lock()
				packetType, data, err := m.readPacket()
				if err != nil {
					log.Printf("Failed to read packet: %s", err)
					return
				}

				if packetType == "GFRA" {
					frame := image.NewGray16(sensorSizes[m.cameraType])
					if len(data) < len(frame.Pix) {
						log.Printf("Invalid thermal image frame: %d words (of %d words expected)", len(data)/2, len(frame.Pix)/2)
						return
					}

					// Drop headers
					frame.Pix = data[len(data)-len(frame.Pix):]
					videoStream <- frame
				}
			}
		}
	}()
	return cancel, videoStream, nil
}

func (m *MI48) getFirmwareVersion() (string, error) {
	firmwareVersion, err := m.readRegister(FW_VERSION)
	if err != nil {
		return "", fmt.Errorf("failed to read firmware version: %w", err)
	}
	return fmt.Sprintf("%d.%d.%d", firmwareVersion[0]>>4&0xF, firmwareVersion[0]&0xF, firmwareVersion[1]), nil
}

func (m *MI48) getCameraID() (string, error) {
	sensorType, err := m.readRegister(SENXOR_ID)
	if err != nil {
		return "", fmt.Errorf("failed to read camera ID: %w", err)
	}
	return fmt.Sprintf("Week: %2d; Year: %4d; Fab: %02X; Serial: %0X", sensorType[1], int(sensorType[0])+2000, sensorType[2], sensorType[3:]), nil
}

func (m *MI48) getCameraType() (uint8, error) {
	cameraType, err := m.readRegister(SENXOR_TYPE)
	if err != nil {
		return 0, fmt.Errorf("failed to get camera type: %w", err)
	}

	return cameraType[0], nil
}

func (m *MI48) writeRegister(reg register, value uint8) error {
	if reg.ReadOnly {
		return fmt.Errorf("register is read-only")
	}

	_, err := m.sendCommand(fmt.Sprintf("WREG%02X%02XXXXX", reg.Address, value))
	if err != nil {
		return fmt.Errorf("failed to write register: %w", err)
	}

	return nil
}

func (m *MI48) readRegister(reg register) ([]byte, error) {
	var responseData = make([]byte, 0)
	for i := uint8(0); i < uint8(reg.Length); i++ {
		response, err := m.sendCommand(fmt.Sprintf("RREG%02XXXXXXX", reg.Address+i))
		if err != nil {
			return []byte{}, fmt.Errorf("failed to read register: %w", err)
		}

		if len(response) == 0 {
			return []byte{}, fmt.Errorf("failed to read register: invalid response length (%d)", len(response))
		}
		responseData = append(responseData, response...)
	}

	value, err := hex.DecodeString(string(responseData))
	if err != nil {
		return []byte{}, fmt.Errorf("failed to decode register value: %w", err)
	}
	return value, nil
}

func (m *MI48) sendCommand(cmd string) (data []byte, err error) {
	cmdType := cmd[0:4]

	// Reformat cmd, include length
	cmd = fmt.Sprintf("   #%04X%s", len(cmd), cmd)

	_, err = m.serialPort.Write([]byte(cmd))
	if err != nil {
		return []byte{}, fmt.Errorf("failed to write to serial port: %w", err)
	}

	// Prevent stream goroutine from interfering with commands
	m.readMutex.Lock()
	defer m.readMutex.Unlock()

	for packetType := ""; packetType != cmdType; packetType, data, err = m.readPacket() {
		if err != nil {
			return []byte{}, fmt.Errorf("failed to read response: %w", err)
		}
	}

	return
}

func (m *MI48) readPacket() (packetType string, data []byte, err error) {
	header := make([]byte, 12)
	for ; string(header)[:4] != "   #"; _, err = io.ReadFull(m.serialPort, header) {
		if err != nil {
			return "", []byte{}, fmt.Errorf("failed to read header from serial port: %w", err)
		}
	}

	header = header[4:]
	packetType = string(header[4:])

	len, err := hex.DecodeString(string(header[:4]))
	if err != nil {
		return "", []byte{}, fmt.Errorf("failed to decode packet length: %w", err)
	}

	dataLength := binary.BigEndian.Uint16(len) - 8

	data = make([]byte, dataLength)
	_, err = io.ReadFull(m.serialPort, data)
	if err != nil {
		return "", []byte{}, fmt.Errorf("failed to read data from serial port: %w", err)
	}

	crc := make([]byte, 4)
	_, err = m.serialPort.Read(crc)
	if err != nil {
		return "", []byte{}, fmt.Errorf("failed to read CRC from serial port: %w", err)
	}

	// TODO check CRC

	return
}

func getSerialPort() (string, error) {
	portDetails, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return "", fmt.Errorf("failed to autodecte MI48 serial port: %w", err)
	}

	for _, port := range portDetails {
		if strings.EqualFold(port.VID, VENDOR_ID) && slices.Contains(PRODUCT_IDs, port.PID) {
			return port.Name, nil
		}
	}

	return "", nil
}
