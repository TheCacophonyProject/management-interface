
export interface FrameInfo {
  Telemetry: Telemetry;
  AppVersion: string;
  BinaryVersion: string;
  Camera: CameraInfo;
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

export interface CameraInfo {
  Brand: string;
  Model: string;
  FPS: number;
  ResX: number;
  ResY: number;
  Firmware: string;
  CameraSerial: number;
}


export interface Frame {
  frameInfo: FrameInfo;
  frame: Uint16Array;
}

interface CameraStats {
  skippedFramesServer: number;
  skippedFramesClient: number;
}

interface CameraState {
  socket: WebSocket | null;
  UUID: number;
  stats: CameraStats;
  prevFrameNum: number;
  heartbeatInterval: number;
}
