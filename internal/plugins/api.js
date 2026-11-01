"use strict";
// The API plugins get, built over the natives of the Go side (native), as
// the web interface's src/plugins/api.ts built it over its own. It returns
// the MixMusicAPI of sdk/index.d.ts, the types published for plugin
// authors, which the type checker holds it to; docs/plugins.md tells how
// to use it. `bun run build:plugins` compiles it to api.js, which the Go
// side runs: a script, not a module, whose value is this function.
(function (native) {
    const done = Promise.resolve();
    const call = (fn) => (...args) => {
        fn(...args);
        return done;
    };
    // The elements a page or a widget shows: plain objects, which plugins may
    // write themselves.
    const elements = {
        text: (text, options) => ({ ...options, type: "text", text: String(text) }),
        heading: (text) => ({ type: "heading", text: String(text) }),
        button: (label, onClick, options) => ({ ...options, type: "button", label: String(label), onClick }),
        link: (label, target) => typeof target === "function" ? { type: "link", label: String(label), onClick: target } : { type: "link", label: String(label), path: String(target) },
        tiles: (children) => ({ type: "tiles", children }),
        tile: (title, onClick, options) => ({ ...options, type: "tile", title: String(title), onClick }),
        row: (children, options) => ({ ...options, type: "row", children }),
        column: (children, options) => ({ ...options, type: "column", children }),
        canvas: (options) => ({ ...options, type: "canvas" }),
        image: (url, options) => ({ ...options, type: "image", url: String(url) }),
        spacer: (size) => ({ type: "spacer", size }),
        toggle: (label, on, onChange) => ({ type: "toggle", label: String(label), on: Boolean(on), onChange }),
        input: (options) => ({ ...options, type: "input" }),
    };
    const getJSON = async (key, fallback) => {
        const raw = await native.storageGet(String(key));
        if (raw == null)
            return fallback;
        try {
            return JSON.parse(raw);
        }
        catch {
            return fallback;
        }
    };
    return Object.freeze({
        version: native.version,
        plugin: native.plugin,
        player: Object.freeze({
            state: () => native.playerState(),
            play: call(native.play),
            pause: call(native.pause),
            toggle: call(native.toggle),
            next: call(native.next),
            previous: call(native.previous),
            seek: call((ms) => native.seek(Number(ms))),
            setVolume: (volume) => native.setVolume(Number(volume)),
            playTracks: call((tracks, start) => native.playTracks(tracks, Number(start ?? 0))),
            playNext: (tracks) => native.playNext(tracks),
            enqueue: (tracks) => native.enqueue(tracks),
            on: (event, listener) => native.on(String(event), listener),
            analyser: () => native.analyser(),
        }),
        catalog: Object.freeze({
            search: (query) => native.search(String(query)),
            album: (id) => native.album(String(id)),
            artist: (id) => native.artist(String(id)),
            playlist: (id) => native.playlist(String(id)),
            radio: (track) => native.radio(track),
            lyrics: (track) => native.lyrics(track),
        }),
        library: Object.freeze({
            isLiked: (id) => native.isLiked(String(id)),
            setLiked: (track, liked) => native.setLiked(track, Boolean(liked)),
            mixes: () => native.mixes(),
            createMix: (title, tracks) => native.createMix(String(title), tracks ?? []),
        }),
        ui: Object.freeze({
            toast: (message, options) => native.toast(String(message), String(options?.tone ?? "info")),
            navigate: (path) => native.navigate(String(path)),
            theme: () => native.theme(),
            addSidebarItem: ({ id, label, icon, page }) => native.addSidebarItem(String(id), String(label), String(icon ?? ""), String(page)),
            addPage: ({ name, title, render }) => native.addPage(String(name), String(title), render),
            addTrackAction: ({ id, label, run }) => native.addTrackAction(String(id), String(label), run),
            addWidget: ({ id, title, render }) => native.addWidget(String(id), String(title), render),
            ...elements,
        }),
        commands: Object.freeze({
            register: ({ id, title, shortcut, run }) => native.registerCommand(String(id), String(title), String(shortcut ?? ""), run),
        }),
        settings: Object.freeze({
            get: () => native.settings(),
        }),
        storage: Object.freeze({
            get: (key) => native.storageGet(String(key)),
            set: (key, value) => native.storageSet(String(key), String(value)),
            delete: (key) => native.storageDelete(String(key)),
            getJSON,
            setJSON: (key, value) => native.storageSet(String(key), JSON.stringify(value)),
        }),
        notify: (title, body) => native.notify(String(title), String(body)),
        openExternal: (url) => native.openExternal(String(url)),
    });
});
