/**
 * Milesight EM300-TH payload decoder (reference codec shipped with the
 * marketplace entry). Returns temperature in °C and humidity in %RH. This is
 * surfaced for the user to view/download; the simulator does not run it yet.
 *
 * @param {number} fPort - the LoRaWAN application port
 * @param {number[]} bytes - the raw uplink payload bytes
 * @returns {{ temperature?: number, humidity?: number }} decoded readings
 */
function decodeUplink(fPort, bytes) {
	const decoded = {};
	for (let i = 0; i < bytes.length; ) {
		const channel = bytes[i++];
		const type = bytes[i++];
		// 0x67: temperature, int16, 0.1 °C
		if (channel === 0x01 && type === 0x67) {
			decoded.temperature = ((bytes[i + 1] << 8) | bytes[i]) / 10;
			i += 2;
		}
		// 0x68: humidity, uint8, 0.5 %RH
		else if (channel === 0x02 && type === 0x68) {
			decoded.humidity = bytes[i] / 2;
			i += 1;
		}
	}
	return decoded;
}
