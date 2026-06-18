/**
 * Milesight VS121 payload decoder (reference codec shipped with the marketplace
 * entry). Returns cumulative in/out passage counts. Surfaced for the user to
 * view/download; the simulator does not run it yet.
 *
 * @param {number} fPort - the LoRaWAN application port
 * @param {number[]} bytes - the raw uplink payload bytes
 * @returns {{ peopleInTotal?: number, peopleOutTotal?: number }} decoded counts
 */
function decodeUplink(fPort, bytes) {
	const decoded = {};
	for (let i = 0; i < bytes.length; ) {
		const channel = bytes[i++];
		const type = bytes[i++];
		if (channel === 0x04 && type === 0xc9) {
			decoded.peopleInTotal = (bytes[i + 1] << 8) | bytes[i];
			i += 2;
		} else if (channel === 0x05 && type === 0x7d) {
			decoded.peopleOutTotal = (bytes[i + 1] << 8) | bytes[i];
			i += 2;
		}
	}
	return decoded;
}
