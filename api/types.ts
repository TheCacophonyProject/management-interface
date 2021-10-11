
export interface PartialFrameInfo {
  Calibration: CalibrationInfo | null;
  Telemetry: Telemetry;
  AppVersion: string;
  BinaryVersion: string;
  Camera: CameraInfo;
}

export interface FrameInfo {
  Calibration: CalibrationInfo;
  Telemetry: Telemetry;
  AppVersion: string;
  BinaryVersion: string;
  Camera: CameraInfo;
}

export interface CalibrationInfo {
  ThermalRefTemp: number;
  SnapshotTime: number;
  TemperatureCelsius: number;
  SnapshotValue: number;
  ThresholdMinFever: number;
  HeadTLX: number;
  HeadTLY: number;
  HeadBLX: number;
  HeadBLY: number;
  HeadTRX: number;
  HeadTRY: number;
  HeadBRX: number;
  HeadBRY: number;
  CalibrationBinaryVersion: string;
  UuidOfUpdater: number;
  UseNormalSound: boolean;
  UseWarningSound: boolean;
  UseErrorSound: boolean;
}

export interface Telemetry {
  TimeOn: number;
  FFCState: string;
  FrameCount: number;
  FrameMean: number;
  TempC: number;
  LastFFCTempC: number;
  LastFFCTime: number;
}
export interface NetworkInterface {
  Name: string;
  IPAddresses: string[] | null;
}

export interface CameraInfo {
  Brand: string;
  Model: string;
  FPS: number;
  ResX: number;
  ResY: number;
  Firmware: string;
  CameraSerial: number;
}
