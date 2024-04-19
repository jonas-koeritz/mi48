package mi48

import "image"

type register struct {
	Address  uint8
	Length   int
	ReadOnly bool
}

var FRAME_MODE = register{0xB1, 1, false}
var FW_VERSION = register{0xB2, 2, true}
var FRAME_RATE = register{0xB4, 1, false}
var POWER_DOWN_1 = register{0xB5, 1, false}
var STATUS = register{0xB6, 1, true}
var CLK_SPEED = register{0xB7, 1, false}
var SENXOR_TYPE = register{0xBA, 1, true}
var SENSITIVITY_FACTOR = register{0xC2, 1, false}
var EMISSIVITY = register{0xCA, 1, false}
var OFFSET_CORR = register{0xCB, 1, false}
var SENXOR_ID = register{0xE0, 6, true}
var FILTER_CONTROL = register{0xD0, 1, false}
var FILTER_SETTING_1_LSB = register{0xD1, 1, false}
var FILTER_SETTING_1_MSB = register{0xD2, 1, false}
var FILTER_SETTING_2 = register{0xD3, 1, false}
var NETD_CONFIG = register{0xD4, 1, false}
var NETD_FACTOR = register{0xD5, 1, false}
var NETD_PIXEL_X = register{0xD6, 1, false}
var NETD_PIXEL_Y = register{0xD7, 1, false}
var USER_FLASH_CTRL = register{0xD8, 1, false}

type frameMode uint8

var GET_SINGLE_FRAME = frameMode(1)
var CONTNIUOUS_STREAM = frameMode(2)
var NO_HEADER = frameMode(32)
var LOW_NETD_ROW_IN_HEADER = frameMode(64)

var cameraTypes = map[uint8]string{
	0: "MI0801 non-MP",
	1: "MI0801",
	2: "MI0301",
	3: "MI0802",
	8: "panther",
}

var sensorSizes = map[uint8]image.Rectangle{
	0: image.Rect(0, 0, 80, 62),
	1: image.Rect(0, 0, 80, 62),
	2: image.Rect(0, 0, 32, 32),
	3: image.Rect(0, 0, 80, 62),
	8: image.Rect(0, 0, 160, 120),
}
