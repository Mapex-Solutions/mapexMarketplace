/**
 * Milesight EM300-TH decoder — ChirpStack v4 flavour. ChirpStack v4 adopted the
 * LoRaWAN Payload Codec API (TS013), so the entry point is decodeUplink(input)
 * and the result is wrapped in { data }. Same byte math as the vendor codec, only
 * the wrapper signature differs from older ChirpStack v3 (Decode(fPort, bytes)).
 *
 * @param {{ bytes: number[], fPort: number }} input - the uplink frame
 * @returns {{ data: { temperature?: number, humidity?: number } }} decoded result
 */
function decodeUplink(input) {
	const bytes = input.bytes;
	const data = {};
	for (let i = 0; i < bytes.length; ) {
		const channel = bytes[i++];
		const type = bytes[i++];
		if (channel === 0x01 && type === 0x67) {
			data.temperature = ((bytes[i + 1] << 8) | bytes[i]) / 10;
			i += 2;
		} else if (channel === 0x02 && type === 0x68) {
			data.humidity = bytes[i] / 2;
			i += 1;
		}
	}
	return { data: data };
}
