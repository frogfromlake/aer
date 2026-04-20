// Solar position approximation — adapted from NOAA's solar geometry algorithm
// (https://gml.noaa.gov/grad/solcalc/calcdetails.html). Returns the
// sun-direction vector in the same Earth-centred Cartesian frame the globe
// mesh uses (+Z toward lon=0 equator, +Y north pole). Accuracy is ~0.5°,
// which is well below the ~5° smoothstep band of the terminator shader.

import { Vector3 } from 'three';

const DEG = Math.PI / 180;

export function sunDirection(unixMs: number, out: Vector3 = new Vector3()): Vector3 {
  const julianDay = unixMs / 86_400_000 + 2_440_587.5;
  const julianCentury = (julianDay - 2_451_545) / 36_525;

  // Geometric mean longitude and anomaly of the sun (degrees).
  const meanLong = (280.46646 + julianCentury * (36_000.76983 + julianCentury * 0.0003032)) % 360;
  const meanAnom = 357.52911 + julianCentury * (35_999.05029 - 0.0001537 * julianCentury);

  // Equation of centre.
  const sinM = Math.sin(meanAnom * DEG);
  const sin2M = Math.sin(2 * meanAnom * DEG);
  const sin3M = Math.sin(3 * meanAnom * DEG);
  const eqCentre =
    sinM * (1.914602 - julianCentury * (0.004817 + 0.000014 * julianCentury)) +
    sin2M * (0.019993 - 0.000101 * julianCentury) +
    sin3M * 0.000289;

  const trueLong = meanLong + eqCentre;

  // Apparent longitude (corrects for nutation/aberration in a single term).
  const omega = 125.04 - 1934.136 * julianCentury;
  const appLong = trueLong - 0.00569 - 0.00478 * Math.sin(omega * DEG);

  // Mean obliquity of the ecliptic (Meeus, simplified).
  const meanObl =
    23 +
    (26 +
      (21.448 - julianCentury * (46.815 + julianCentury * (0.00059 - julianCentury * 0.001813))) /
        60) /
      60;
  const obl = meanObl + 0.00256 * Math.cos(omega * DEG);

  // Declination of the sun.
  const declination = Math.asin(Math.sin(obl * DEG) * Math.sin(appLong * DEG));

  // Greenwich Mean Sidereal Time → sub-solar longitude.
  const gmst =
    (280.46061837 +
      360.98564736629 * (julianDay - 2_451_545) +
      julianCentury * julianCentury * (0.000387933 - julianCentury / 38_710_000)) %
    360;
  // Right ascension of the sun (atan2 with longitude correction).
  const ra =
    Math.atan2(Math.cos(obl * DEG) * Math.sin(appLong * DEG), Math.cos(appLong * DEG)) / DEG;
  const subSolarLon = ((ra - gmst + 540) % 360) - 180;

  // Convert (lon, declination) → unit Cartesian using the same projection as
  // the engine's landmass mesh: lon=0 → +Z, north → +Y.
  const lonRad = subSolarLon * DEG;
  const cosDec = Math.cos(declination);
  out.set(cosDec * Math.sin(lonRad), Math.sin(declination), cosDec * Math.cos(lonRad));
  return out;
}
