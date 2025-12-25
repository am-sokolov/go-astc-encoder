package astc

// quantMethod is an ASTC integer-sequence quantization mode.
//
// The numeric values are specified by the ASTC format and must not be reordered.
type quantMethod uint8

const (
	quant2   quantMethod = 0
	quant3   quantMethod = 1
	quant4   quantMethod = 2
	quant5   quantMethod = 3
	quant6   quantMethod = 4
	quant8   quantMethod = 5
	quant10  quantMethod = 6
	quant12  quantMethod = 7
	quant16  quantMethod = 8
	quant20  quantMethod = 9
	quant24  quantMethod = 10
	quant32  quantMethod = 11
	quant40  quantMethod = 12
	quant48  quantMethod = 13
	quant64  quantMethod = 14
	quant80  quantMethod = 15
	quant96  quantMethod = 16
	quant128 quantMethod = 17
	quant160 quantMethod = 18
	quant192 quantMethod = 19
	quant256 quantMethod = 20
)
