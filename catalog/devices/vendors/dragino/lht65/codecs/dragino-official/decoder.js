/**
 * Dragino LHT65 payload decoder (reference codec shipped with the marketplace
 * entry). Surfaced for the user to view/download; the simulator does not run it
 * yet.
 *
 * @param {number} fPort - the LoRaWAN application port
 * @param {number[]} bytes - the raw uplink payload bytes
 * @returns {{ humidity?: number, temperature?: number }} decoded readings
 */
function decodeUplink(fPort, bytes) {
	const humidity = ((bytes[2] << 8) | bytes[3]) / 10;
	const raw = (bytes[5] << 8) | bytes[6];
	const temperature = raw === 0x7fff ? null : (raw & 0x8000 ? raw - 0x10000 : raw) / 100;
	return { humidity, temperature };
}
