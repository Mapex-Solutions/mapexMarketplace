/**
 * Elsys LoRaWAN payload decoder.
 *
 * Clean-room implementation of the public Elsys sensor payload protocol
 * (TLV: type byte = [NOB:2][STYPE:6], followed by the measurement bytes).
 * Source spec: https://elsys.se/public/documents/Sensor_payload.pdf
 *
 * Covers one generic format shared by the whole Elsys range (ERS/ELT/EMS/ECO).
 */
var TYPE = {
  0x01: ["temperature", 2, function (b, i) { return s16(b, i) / 10; }],
  0x02: ["humidity", 1, function (b, i) { return b[i]; }],
  0x03: ["acceleration", 3, function (b, i) { return { x: s8(b[i]), y: s8(b[i + 1]), z: s8(b[i + 2]) }; }],
  0x04: ["light", 2, function (b, i) { return u16(b, i); }],
  0x05: ["motion", 1, function (b, i) { return b[i]; }],
  0x06: ["co2", 2, function (b, i) { return u16(b, i); }],
  0x07: ["vdd", 2, function (b, i) { return u16(b, i); }],
  0x08: ["analog1", 2, function (b, i) { return u16(b, i); }],
  0x09: ["gps", 6, function (b, i) { return { lat: s24(b, i) / 10000, long: s24(b, i + 3) / 10000 }; }],
  0x0a: ["pulse1", 2, function (b, i) { return u16(b, i); }],
  0x0b: ["pulseAbs1", 4, function (b, i) { return u32(b, i); }],
  0x0c: ["externalTemperature1", 2, function (b, i) { return s16(b, i) / 10; }],
  0x0d: ["digital", 1, function (b, i) { return b[i]; }],
  0x0e: ["distance", 2, function (b, i) { return u16(b, i); }],
  0x0f: ["accMotion", 1, function (b, i) { return b[i]; }],
  0x10: ["irTemperature", 4, function (b, i) { return { internal: s16(b, i) / 10, external: s16(b, i + 2) / 10 }; }],
  0x11: ["occupancy", 1, function (b, i) { return b[i]; }],
  0x12: ["waterleak", 1, function (b, i) { return b[i]; }],
  0x13: ["grideye", 65, function (b, i) { return null; }],
  0x14: ["pressure", 4, function (b, i) { return u32(b, i) / 1000; }],
  0x15: ["sound", 2, function (b, i) { return { peak: b[i], avg: b[i + 1] }; }],
  0x16: ["pulse2", 2, function (b, i) { return u16(b, i); }],
  0x17: ["pulseAbs2", 4, function (b, i) { return u32(b, i); }],
  0x18: ["analog2", 2, function (b, i) { return u16(b, i); }],
  0x19: ["externalTemperature2", 2, function (b, i) { return s16(b, i) / 10; }],
  0x1a: ["digital2", 1, function (b, i) { return b[i]; }],
  0x1b: ["analogUv", 4, function (b, i) { return s32(b, i); }],
};
function s8(v) { return v > 127 ? v - 256 : v; }
function u16(b, i) { return (b[i] << 8) | b[i + 1]; }
function s16(b, i) { var v = u16(b, i); return v > 32767 ? v - 65536 : v; }
function s24(b, i) { var v = (b[i] << 16) | (b[i + 1] << 8) | b[i + 2]; return v > 8388607 ? v - 16777216 : v; }
function u32(b, i) { return (b[i] * 0x1000000) + (b[i + 1] << 16) + (b[i + 2] << 8) + b[i + 3]; }
function s32(b, i) { var v = u32(b, i); return v > 2147483647 ? v - 4294967296 : v; }

function decodeElsys(bytes) {
  var o = {};
  for (var i = 0; i < bytes.length; ) {
    var t = bytes[i++];
    var nob = (t >> 6) & 0x03;          // number of offset bytes
    var stype = t & 0x3f;
    var def = TYPE[stype];
    if (!def) break;                    // unknown type → stop (avoid garbage)
    var size = def[1];
    var val = def[2](bytes, i);
    if (val !== null) o[def[0]] = val;
    i += size + (nob === 3 ? 4 : nob);  // skip data + offset bytes (NOB: 0/1/2/4)
  }
  return o;
}
function decodeUplink(input) { return { data: decodeElsys(input.bytes) }; }
function Decode(fPort, bytes) { return decodeElsys(bytes); }
function Decoder(bytes, port) { return decodeElsys(bytes); }
