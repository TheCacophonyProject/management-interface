import { FrameInfo, Frame } from "../../api/types";

export const BlobReader = (function (): {
  arrayBuffer: (blob: Blob) => Promise<ArrayBuffer>;
} {
  // For comparability with older browsers/iOS that don't yet support arrayBuffer()
  // directly off the blob object
  const arrayBuffer: (blob: Blob) => Promise<ArrayBuffer> =
    "arrayBuffer" in Blob.prototype &&
    typeof (Blob.prototype as Blob)["arrayBuffer"] === "function"
      ? (blob: Blob) => blob["arrayBuffer"]()
      : (blob: Blob) =>
          new Promise((resolve, reject) => {
            const fileReader = new FileReader();
            fileReader.addEventListener("load", () => {
              resolve(fileReader.result as ArrayBuffer);
            });
            fileReader.addEventListener("error", () => {
              reject();
            });
            fileReader.readAsArrayBuffer(blob);
          });

  return {
    arrayBuffer,
  };
})();
export enum CameraConnectionState {
  Connecting,
  Connected,
  Disconnected,
}

const UUID = new Date().getTime();

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

let snapshotCount = 0;
let snapshotLimit = 200;
let cameraConnection: CameraConnection;
function restartCameraViewing() {
  document.getElementById("snapshot-stopped")!.style.display = "none";
  document.getElementById("canvas")!.style.display = "";
  snapshotCount = 0;
  if (cameraConnection) {
    cameraConnection.connect();
  } else {
    cameraConnection = new CameraConnection(
      window.location.hostname,
      window.location.port,
      processFrame,
      onConnectionStateChange
    );
  }
}

window.onload = function () {
  const urlParams = new URLSearchParams(window.location.search);
  if (urlParams.get("timeout") == "off") {
    snapshotLimit = Number.MAX_SAFE_INTEGER;
  }
  document.getElementById("snapshot-restart")!.onclick = restartCameraViewing;
  cameraConnection = new CameraConnection(
    window.location.hostname,
    window.location.port,
    processFrame,
    onConnectionStateChange
  );
};

function stopSnapshots(message: string) {
  if (cameraConnection) {
    cameraConnection.close();
  }
  document.getElementById("snapshot-stopped-message")!.innerText = message;
  document.getElementById("snapshot-stopped")!.style.display = "";
  document.getElementById("canvas")!.style.display = "none";
  console.log("stopping snappes");
}

function onConnectionStateChange(connectionState: CameraConnectionState) {}

async function processFrame(frame: Frame) {
  const canvas = document.getElementById("canvas") as HTMLCanvasElement;
  if (canvas == null) {
    return;
  }
  const context = canvas.getContext("2d") as CanvasRenderingContext2D;
  const imgData = context.getImageData(
    0,
    0,
    frame.frameInfo.Camera.ResX,
    frame.frameInfo.Camera.ResY
  );
  const max = Math.max(...frame.frame);
  const min = Math.min(...frame.frame);
  const range = max - min;
  let maxI = 0;
  for (let i = 0; i < frame.frame.length; i++) {
    const pix = Math.min(255, ((frame.frame[i] - min) / range) * 255.0);
    let index = i * 4;
    imgData.data[index] = pix;
    imgData.data[index + 1] = pix;
    imgData.data[index + 2] = pix;
    imgData.data[index + 3] = 255;
    maxI = index;
  }
  context.putImageData(imgData, 0, 0);
  document.getElementById(
    "snapshot-frame"
  )!.innerText = `frame ${frame.frameInfo.Telemetry.FrameCount}`;
}

export class CameraConnection {
  private closing: boolean;

  constructor(
    public host: string,
    public port: string,
    public onFrame: (frame: Frame) => void,
    public onConnectionStateChange: (
      connectionState: CameraConnectionState
    ) => void
  ) {
    this.closing = false;
    this.connect();
  }
  private state: CameraState = {
    socket: null,
    UUID: new Date().getTime(),
    stats: {
      skippedFramesServer: 0,
      skippedFramesClient: 0,
    },
    prevFrameNum: -1,
    heartbeatInterval: 0,
  };
  close() {
    clearInterval(this.state.heartbeatInterval);
    this.closing = true;
    if (this.state.socket) {
      this.state.socket.close();
    }
  }
  retryConnection(retryTime: number) {
    if (this.closing) {
      return;
    }
    if (retryTime > 0) {
      setTimeout(() => this.retryConnection(retryTime - 1), 1000);
    } else {
      this.connect();
    }
  }
  register() {
    if (this.state.socket !== null) {
      if (this.state.socket.readyState === WebSocket.OPEN) {
        // We are waiting for frames now.
        this.state.socket.send(
          JSON.stringify({
            type: "Register",
            data: navigator.userAgent,
            uuid: UUID,
          })
        );
        this.onConnectionStateChange(CameraConnectionState.Connected);

        this.state.heartbeatInterval = setInterval(() => {
          this.state.socket &&
            this.state.socket.send(
              JSON.stringify({
                type: "Heartbeat",
                uuid: UUID,
              })
            );
        }, 5000) as unknown as number;
      } else {
        setTimeout(this.register.bind(this), 100);
      }
    }
  }
  connect() {
    this.closing = false;
    this.state.socket = new WebSocket(`ws://${this.host}:${this.port}/ws`);
    this.onConnectionStateChange(CameraConnectionState.Connecting);
    this.state.socket.addEventListener("error", (e) => {
      console.warn("Websocket Connection error", e);
      //...
    });
    // Connection opened
    this.state.socket.addEventListener("open", this.register.bind(this));
    this.state.socket.addEventListener("close", () => {
      // When we do reconnect, we need to treat it as a new connection
      console.warn("Websocket closed");
      this.state.socket = null;
      this.onConnectionStateChange(CameraConnectionState.Disconnected);
      clearInterval(this.state.heartbeatInterval);
      this.retryConnection(5);
    });
    this.state.socket.addEventListener("message", async (event) => {
      if (event.data instanceof Blob) {
        this.onFrame((await this.parseFrame(event.data as Blob)) as Frame);
      } else {
        console.log("got message", event.data);
      }
      snapshotCount++;

      if (snapshotCount > snapshotLimit) {
        stopSnapshots("Timeout for camera viewing.");
      }
    });
  }
  async parseFrame(
    blob: Blob
  ): Promise<{ frameInfo: FrameInfo; frame: Uint16Array } | null> {
    // NOTE(jon): On iOS. it seems slow to do multiple fetches from the blob, so let's do it all at once.
    const data = await BlobReader.arrayBuffer(blob);
    const frameInfoLength = new Uint16Array(data.slice(0, 2))[0];
    const frameStartOffset = 2 + frameInfoLength;
    try {
      const frameInfo = JSON.parse(
        String.fromCharCode(...new Uint8Array(data.slice(2, frameStartOffset)))
      ) as FrameInfo;
      const frameNumber = frameInfo.Telemetry.FrameCount;
      if (frameNumber % 20 === 0) {
        performance.clearMarks();
        performance.clearMeasures();
        performance.clearResourceTimings();
      }
      performance.mark(`start frame ${frameNumber}`);
      if (
        this.state.prevFrameNum !== -1 &&
        this.state.prevFrameNum + 1 !== frameInfo.Telemetry.FrameCount
      ) {
        this.state.stats.skippedFramesServer +=
          frameInfo.Telemetry.FrameCount - this.state.prevFrameNum;
        // Work out an fps counter.
      }
      this.state.prevFrameNum = frameInfo.Telemetry.FrameCount;
      const frameSizeInBytes =
        frameInfo.Camera.ResX * frameInfo.Camera.ResY * 2;
      // TODO(jon): Some perf optimisations here.
      const frame = new Uint16Array(
        data.slice(frameStartOffset, frameStartOffset + frameSizeInBytes)
      );
      return {
        frameInfo,
        frame,
      };
    } catch (e) {
      console.error("Malformed JSON payload", e);
    }
    return null;
  }
}
