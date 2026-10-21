// Spectrum: a radial spectrum of what plays, as a widget of the now playing
// page. It reads the analyser of MixMusic's own player, so it moves for
// tracks that play in MixMusic, not on Spotify Connect devices.

export default function activate(api) {
  api.ui.addWidget({
    id: "radial",
    title: "spectrum",
    render(el) {
      const canvas = document.createElement("canvas");
      canvas.style.cssText = "width: 100%; max-width: 360px; aspect-ratio: 1; display: block";
      el.append(canvas);
      const ctx = canvas.getContext("2d");
      const reduce = matchMedia("(prefers-reduced-motion: reduce)").matches;
      let frame = 0;
      let data = new Uint8Array(0);

      const draw = () => {
        frame = requestAnimationFrame(draw);
        const dpr = devicePixelRatio;
        const size = canvas.clientWidth * dpr;
        if (canvas.width !== size) canvas.width = canvas.height = size;
        ctx.clearRect(0, 0, size, size);
        const accent = getComputedStyle(document.documentElement).getPropertyValue("--mm-accent").trim() || "#00bde3";
        const analyser = api.player.analyser();
        const playing = api.player.state().playing;
        const bars = 96;
        const inner = size * 0.22;
        const reach = size * 0.26;
        if (analyser && playing) {
          if (data.length !== analyser.frequencyBinCount) data = new Uint8Array(analyser.frequencyBinCount);
          analyser.getByteFrequencyData(data);
        } else {
          data.fill(0);
        }
        ctx.save();
        ctx.translate(size / 2, size / 2);
        ctx.strokeStyle = accent;
        ctx.lineWidth = Math.max(2, (2 * Math.PI * inner) / bars - 2 * dpr);
        ctx.lineCap = "butt";
        for (let i = 0; i < bars; i++) {
          // Mirror the lower bins around the circle, where the music is.
          const half = i < bars / 2 ? i : bars - i;
          const bin = Math.floor(Math.pow(half / (bars / 2), 1.5) * data.length * 0.7);
          const v = (data[bin] ?? 0) / 255;
          const a = (i / bars) * Math.PI * 2 - Math.PI / 2;
          const r2 = inner + 3 * dpr + v * reach;
          ctx.beginPath();
          ctx.moveTo(Math.cos(a) * inner, Math.sin(a) * inner);
          ctx.lineTo(Math.cos(a) * r2, Math.sin(a) * r2);
          ctx.stroke();
        }
        ctx.restore();
        if (reduce) cancelAnimationFrame(frame);
      };
      draw();
      return () => cancelAnimationFrame(frame);
    },
  });
}
