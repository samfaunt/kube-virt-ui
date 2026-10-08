// Minimal typings for the parts of noVNC's RFB client used here.
declare module '@novnc/novnc' {
  export default class RFB extends EventTarget {
    constructor(target: HTMLElement, url: string, options?: { wsProtocols?: string[] })
    scaleViewport: boolean
    resizeSession: boolean
    background: string
    focusOnClick: boolean
    disconnect(): void
    sendCtrlAltDel(): void
    focus(): void
  }
}
