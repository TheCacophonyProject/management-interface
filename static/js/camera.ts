import { FrameInfo, Frame, Region, CameraInfo } from "../../api/types";

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

const colours = ["#ff0000", "#00ff00", "#ffff00", "#80ffff"];
let snapshotCount = 0;
let snapshotLimit = 200;
let cameraConnection: CameraConnection;
function restartCameraViewing() {
  document.getElementById("snapshot-stopped")!.style.display = "none";
  document.getElementById("frameCanvas")!.style.display = "";
  document.getElementById("trackCanvas")!.style.display = "";

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

async function triggerTrap() {
  document.getElementById("trigger-trap")!.innerText = 'Triggering trap';
  document.getElementById("trigger-trap")!.setAttribute("disabled", "true");
  console.log("triggering trap");
  fetch('/api/trigger-trap', {
    method: 'PUT',
  headers: {
    'Authorization': 'Basic YWRtaW46ZmVhdGhlcnM='
  }})

  .then(response => console.log(response))
  .then(data => console.log(data))
  .catch(error => console.error(error))
  //TODO handle errors better and check that recording was made properly instead of just waiting..
  await new Promise(r => setTimeout(r, 3000));
  document.getElementById("trigger-trap")!.removeAttribute("disabled");
  document.getElementById("trigger-trap")!.innerText = 'Trigger trap';
}

window.onload = function () {
  const urlParams = new URLSearchParams(window.location.search);
  if (urlParams.get("timeout") == "off") {
    snapshotLimit = Number.MAX_SAFE_INTEGER;
  }
  document.getElementById("snapshot-restart")!.onclick = restartCameraViewing;
  document.getElementById("trigger-trap")!.onclick = triggerTrap;
  document.getElementById("take-snapshot-recording")!.onclick = takeTestRecording;
  cameraConnection = new CameraConnection(
    window.location.hostname,
    window.location.port,
    processFrame,
    onConnectionStateChange
  );
};

async function takeTestRecording() {
  document.getElementById("take-snapshot-recording")!.innerText = 'Making a test recording';
  document.getElementById("take-snapshot-recording")!.setAttribute("disabled", "true");
  console.log("making a test recording");
  fetch('/api/camera/snapshot-recording', {
    method: 'PUT',
  headers: {
    'Authorization': 'Basic YWRtaW46ZmVhdGhlcnM='
  }})

  .then(response => console.log(response))
  .then(data => console.log(data))
  .catch(error => console.error(error))
  //TODO handle errors better and check that recording was made properly instead of just waiting..
  await new Promise(r => setTimeout(r, 3000));
  document.getElementById("take-snapshot-recording")!.removeAttribute("disabled");
  document.getElementById("take-snapshot-recording")!.innerText = 'Take test recording';
}

function stopSnapshots(message: string) {
  if (cameraConnection) {
    cameraConnection.close();
  }
  document.getElementById("snapshot-stopped-message")!.innerText = message;
  document.getElementById("snapshot-stopped")!.style.display = "";
  document.getElementById("frameCanvas")!.style.display = "none";
  document.getElementById("trackCanvas")!.style.display = "none";
}

function onConnectionStateChange(connectionState: CameraConnectionState) {}

function drawRectWithText(
  context: CanvasRenderingContext2D,
  camera: CameraInfo,
  region: Region,
  what: string | null,
  trackIndex: number
): void {
  const lineWidth = 1;
  const outlineWidth = lineWidth + 4;
  const halfOutlineWidth = outlineWidth / 2;

  const x = Math.max(halfOutlineWidth, Math.round(region.x) - halfOutlineWidth);
  const y = Math.max(halfOutlineWidth, Math.round(region.y) - halfOutlineWidth);
  const width = Math.round(
    Math.min(context.canvas.width - region.x, Math.round(region.width))
  );
  const height = Math.round(
    Math.min(context.canvas.height - region.y, Math.round(region.height))
  );
  context.lineJoin = "round";
  context.lineWidth = outlineWidth;
  context.strokeStyle = `rgba(0, 0, 0,  0.5)`;
  context.beginPath();
  context.strokeRect(x, y, width, height);
  context.strokeStyle = colours[trackIndex % colours.length];
  context.lineWidth = lineWidth;
  context.beginPath();
  context.strokeRect(x, y, width, height);
  // If exporting, show all the best guess animal tags, if not unknown
  if (what !== null) {
    const text = what;
    const textHeight = 9;
    const textWidth = context.measureText(text).width;
    const marginX = 2;
    const marginTop = 2;
    let textX =
      Math.min(context.canvas.width, region.x) - (textWidth + marginX);
    let textY = region.y + region.height + textHeight + marginTop;
    // Make sure the text doesn't get clipped off if the box is near the frame edges
    if (textY + textHeight > context.canvas.height) {
      textY = region.y - textHeight;
    }
    if (textX < 0) {
      textX = region.x + marginX;
    }
    context.font = "13px sans-serif";
    context.lineWidth = 4;
    context.strokeStyle = "rgba(0, 0, 0, 0.5)";
    context.strokeText(text, textX, textY);
    context.fillStyle = "white";
    context.fillText(text, textX, textY);
  }
}

async function processFrame(frame: Frame) {
  const canvas = document.getElementById("frameCanvas") as HTMLCanvasElement;

  const trackCanvas = document.getElementById(
    "trackCanvas"
  ) as HTMLCanvasElement;
  if (canvas == null) {
    return;
  }
  if( canvas.width !=  frame.frameInfo.Camera.ResX){
    canvas.width =     frame.frameInfo.Camera.ResX
    trackCanvas.width =     frame.frameInfo.Camera.ResX
  }
  if(canvas.height !=  frame.frameInfo.Camera.ResY){
    canvas.height =     frame.frameInfo.Camera.ResY
    trackCanvas.height =     frame.frameInfo.Camera.ResY
  }
  const context = canvas.getContext("2d") as CanvasRenderingContext2D;
  const imgData = context.getImageData(
    0,
    0,
    frame.frameInfo.Camera.ResX,
    frame.frameInfo.Camera.ResY
  );
  //  gp hack to see if ir camera, dbus from python makes dictionary have to be all int type
  let irCamera = frame.frameInfo.Camera.ResX >= 640;
  if(irCamera){
    document.getElementById("trigger-trap")!.style.display = "";
  }else{
    document.getElementById("trigger-trap")!.style.display = "none";
  }
  let max=0;
  let min=0;
  let range=0;
  if (!irCamera){
    max = Math.max(...frame.frame);
    min = Math.min(...frame.frame);
    range = max - min;
  }
  let maxI = 0;
  for (let i = 0; i < frame.frame.length; i++) {
    let pix = 0
    if(irCamera){
      pix = frame.frame[i]
    }else{
       pix = Math.min(255, ((frame.frame[i] - min) / range) * 255.0);
    }
    let index = i * 4;
    imgData.data[index] = pix;
    imgData.data[index + 1] = pix;
    imgData.data[index + 2] = pix;
    imgData.data[index + 3] = 255;
    maxI = index;
  }
  context.putImageData(imgData, 0, 0);

  const trackContext = trackCanvas.getContext("2d") as CanvasRenderingContext2D;
  trackContext.clearRect(0, 0, trackCanvas.width, trackCanvas.height);

  let index = 0;
  if (frame.frameInfo.Tracks) {
    for (const track of frame.frameInfo.Tracks) {
      let what = null;
      if (track.predictions && track.predictions.length > 0) {
        what = track.predictions[0].label;
      }
      drawRectWithText(
        trackContext,
        frame.frameInfo.Camera,
        track.positions[track.positions.length - 1],
        what,
        index
      );
      index += 1;
    }
  }
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
