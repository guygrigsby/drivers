// Package ssd1306 implements a driver forthe SSD1306 led matrix controller, it comes in various colors and screen sizes.
//
// Datasheet: https://cdn-shop.adafruit.com/datasheets/SSD1306.pdf
//
package ssd1306

import (
	"errors"
	"machine"
	"time"
)

// Device wraps an SPI connection.
type Device struct {
	bus        Buser
	buffer     []byte
	width      int16
	height     int16
	bufferSize int16
	vccState   VccMode
}

// Config is the configuration for the display
type Config struct {
	Width    int16
	Height   int16
	VccState VccMode
	Address  uint16
}

type I2CBus struct {
	wire    machine.I2C
	Address uint16
}

type SPIBus struct {
	wire     machine.SPI
	dcPin    machine.Pin
	resetPin machine.Pin
	csPin    machine.Pin
}

type Buser interface {
	configure()
	tx(data []byte, isCommand bool)
	setAddress(address uint16)
}

type VccMode uint8

// NewI2C creates a new SSD1306 connection. The I2C wire must already be configured.
func NewI2C(bus machine.I2C) Device {
	return Device{
		bus: &I2CBus{
			wire:    bus,
			Address: Address,
		},
	}
}

// NewSPI creates a new SSD1306 connection. The SPI wire must already be configured.
func NewSPI(bus machine.SPI, dcPin, resetPin, csPin machine.Pin) Device {
	dcPin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	resetPin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	csPin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	return Device{
		bus: &SPIBus{
			wire:     bus,
			dcPin:    dcPin,
			resetPin: resetPin,
			csPin:    csPin,
		},
	}
}

// Configure initializes the display with default configuration
func (d *Device) Configure(cfg Config) {
	if cfg.Width != 0 {
		d.width = cfg.Width
	} else {
		d.width = 128
	}
	if cfg.Height != 0 {
		d.height = cfg.Height
	} else {
		d.height = 64
	}
	if cfg.Address != 0 {
		d.bus.setAddress(cfg.Address)
	}
	if cfg.VccState != 0 {
		d.vccState = cfg.VccState
	} else {
		d.vccState = SWITCHCAPVCC
	}
	d.bufferSize = d.width * d.height / 8
	d.buffer = make([]byte, d.bufferSize)

	d.bus.configure()

	time.Sleep(100 * time.Nanosecond)
	d.Command(DISPLAYOFF)
	d.Command(SETDISPLAYCLOCKDIV)
	d.Command(0x80)
	d.Command(SETMULTIPLEX)
	d.Command(uint8(d.height - 1))
	d.Command(SETDISPLAYOFFSET)
	d.Command(0x0)
	d.Command(SETSTARTLINE | 0x0)
	d.Command(CHARGEPUMP)
	if d.vccState == EXTERNALVCC {
		d.Command(0x10)
	} else {
		d.Command(0x14)
	}
	d.Command(MEMORYMODE)
	d.Command(0x00)
	d.Command(SEGREMAP | 0x1)
	d.Command(COMSCANDEC)

	if (d.width == 128 && d.height == 64) || (d.width == 64 && d.height == 48) { // 128x64 or 64x48
		d.Command(SETCOMPINS)
		d.Command(0x12)
		d.Command(SETCONTRAST)
		if d.vccState == EXTERNALVCC {
			d.Command(0x9F)
		} else {
			d.Command(0xCF)
		}
	} else if d.width == 128 && d.height == 32 { // 128x32
		d.Command(SETCOMPINS)
		d.Command(0x02)
		d.Command(SETCONTRAST)
		d.Command(0x8F)
	} else if d.width == 96 && d.height == 16 { // 96x16
		d.Command(SETCOMPINS)
		d.Command(0x2)
		d.Command(SETCONTRAST)
		if d.vccState == EXTERNALVCC {
			d.Command(0x10)
		} else {
			d.Command(0xAF)
		}
	} else {
		// fail silently, it might work
		println("there's no configuration for this display's size")
	}

	d.Command(SETPRECHARGE)
	if d.vccState == EXTERNALVCC {
		d.Command(0x22)
	} else {
		d.Command(0xF1)
	}
	d.Command(SETVCOMDETECT)
	d.Command(0x40)
	d.Command(DISPLAYALLON_RESUME)
	d.Command(NORMALDISPLAY)
	d.Command(DEACTIVATE_SCROLL)
	d.Command(DISPLAYON)
}

// ClearBuffer clears the image buffer
func (d *Device) ClearBuffer() {
	for i := int16(0); i < d.bufferSize; i++ {
		d.buffer[i] = 0
	}
}

// ClearDisplay clears the image buffer and clear the display
func (d *Device) ClearDisplay() {
	d.ClearBuffer()
	d.Display()
}

// Display sends the whole buffer to the screen
func (d *Device) Display() error {
	// In the 128x64 (SPI) screen resetting to 0x0 after 128 times corrupt the buffer
	// Since we're printing the whole buffer, avoid resetting it
	if d.width != 128 || d.height != 64 {
		d.Command(COLUMNADDR)
		d.Command(0)
		d.Command(uint8(d.width - 1))
		d.Command(PAGEADDR)
		d.Command(0)
		d.Command(uint8(d.height/8) - 1)
	}

	d.Tx(d.buffer, false)
	return nil
}

// SetBuffer changes the whole buffer at once
func (d *Device) SetBuffer(buffer []byte) error {
	if int16(len(buffer)) != d.bufferSize {
		//return ErrBuffer
		return errors.New("wrong size buffer")
	}
	for i := int16(0); i < d.bufferSize; i++ {
		d.buffer[i] = buffer[i]
	}
	return nil
}

// Command sends a command to the display
func (d *Device) Command(command uint8) {
	d.bus.tx([]byte{command}, true)
}

// setAddress sets the address to the I2C bus
func (b *I2CBus) setAddress(address uint16) {
	b.Address = address
}

// setAddress does nothing, but it's required to avoid reflection
// func (b *SPIBus) setAddress(address uint16) {
// 	// do nothing
// 	println("trying to Configure an address on a SPI device")
// }

// configure does nothing, but it's required to avoid reflection
func (b *I2CBus) configure() {}

// configure configures some pins with the SPI bus
func (b *SPIBus) configure() {
	b.csPin.Low()
	b.dcPin.Low()
	b.resetPin.Low()

	b.resetPin.High()
	time.Sleep(1 * time.Millisecond)
	b.resetPin.Low()
	time.Sleep(10 * time.Millisecond)
	b.resetPin.High()
}

// Tx sends data to the display
func (d *Device) Tx(data []byte, isCommand bool) {
	d.bus.tx(data, isCommand)
}

// tx sends data to the display (I2CBus implementation)
func (b *I2CBus) tx(data []byte, isCommand bool) {
	if isCommand {
		b.wire.WriteRegister(uint8(b.Address), 0x00, data)
	} else {
		b.wire.WriteRegister(uint8(b.Address), 0x40, data)
	}
}

// tx sends data to the display (SPIBus implementation)
func (b *SPIBus) tx(data []byte, isCommand bool) {
	if isCommand {
		b.csPin.High()
		time.Sleep(1 * time.Millisecond)
		b.dcPin.Low()
		b.csPin.Low()

		b.wire.Tx(data, nil)
		b.csPin.High()
	} else {
		b.csPin.High()
		time.Sleep(1 * time.Millisecond)
		b.dcPin.High()
		b.csPin.Low()

		b.wire.Tx(data, nil)
		b.csPin.High()
	}
}

// Size returns the current size of the display.
func (d *Device) Size() (w, h int16) {
	return d.width, d.height
}
