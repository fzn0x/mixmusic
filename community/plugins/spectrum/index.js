// Spectrum: a radial spectrum of what plays, as a widget of the now playing
// page. It reads MixMusic's analyser, so it moves for what MixMusic hears:
// tracks that play in MixMusic, and on Windows Spotify in the Spotify app on
// this computer, not on other devices.
export default function activate(api) {
    let data = new Uint8Array(0);
    api.ui.addWidget({
        id: "radial",
        title: "spectrum",
        render(view) {
            view.set(api.ui.canvas({
                width: 360,
                height: 360,
                animate: true,
                draw(ctx, width, height) {
                    const size = Math.min(width, height);
                    const analyser = api.player.analyser();
                    const playing = api.player.state().playing;
                    const bars = 96;
                    const inner = size * 0.22;
                    const reach = size * 0.26;
                    if (analyser && playing) {
                        if (data.length !== analyser.frequencyBinCount)
                            data = new Uint8Array(analyser.frequencyBinCount);
                        analyser.getByteFrequencyData(data);
                    }
                    else {
                        data.fill(0);
                    }
                    ctx.clearRect(0, 0, width, height);
                    ctx.save();
                    ctx.translate(width / 2, height / 2);
                    ctx.strokeStyle = api.ui.theme().accent;
                    ctx.lineWidth = Math.max(2, (2 * Math.PI * inner) / bars - 2);
                    ctx.beginPath();
                    for (let i = 0; i < bars; i++) {
                        // Mirror the lower bins around the circle, where the music is.
                        const half = i < bars / 2 ? i : bars - i;
                        const bin = Math.floor(Math.pow(half / (bars / 2), 1.5) * data.length * 0.7);
                        const v = (data[bin] ?? 0) / 255;
                        const a = (i / bars) * Math.PI * 2 - Math.PI / 2;
                        const r2 = inner + 3 + v * reach;
                        ctx.moveTo(Math.cos(a) * inner, Math.sin(a) * inner);
                        ctx.lineTo(Math.cos(a) * r2, Math.sin(a) * r2);
                    }
                    ctx.stroke();
                    ctx.restore();
                },
            }));
        },
    });
}
