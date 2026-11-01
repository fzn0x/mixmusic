# Spotify

MixMusic reads your Spotify library and searches Spotify through the Spotify
Web API, and plays Spotify tracks itself, on Spotify Connect, or from their
recording on YouTube Music.

## Sign in

Spotify lets each app it does not review serve only up to five accounts its
owner adds, and only while the owner has Spotify Premium, so you sign in with
an app of your own. It takes two minutes:

1. Open the [Spotify developer dashboard](https://developer.spotify.com/dashboard),
   signed in with a Spotify account that has Premium, and create an app. Any
   name and description do; choose the **Web API**.
2. In the app's settings, add this redirect URI exactly:

   ```
   http://127.0.0.1:47823/callback
   ```

3. In the app's settings, under **Users Management**, add your name and the
   e-mail address of the Spotify account you sign in to MixMusic with, as
   your Spotify account page shows it.
4. Copy the app's **Client ID**.
5. In MixMusic, open Settings › Accounts, paste the Client ID and choose
   **sign in to spotify**. Your browser opens Spotify's sign-in page; once you
   agree, it sends you back to MixMusic.

To let a friend sign in with your app, add their name and Spotify e-mail
address under **Users Management** too.

### When Spotify refuses the sign-in

Spotify can let you sign in and then refuse the app the account's data.
Settings › Accounts says which refusal it was:

- **This Spotify account is not one of the app's users.** Add it under
  **Users Management** in the app's settings, with the e-mail address of the
  account you signed in with. Your browser may be signed in to Spotify with
  another account than the app's: Spotify's sign-in page shows which, and
  **Not you?** changes it.
- **The app's owner has no Spotify Premium.** Subscribe with the account that
  made the app, or make one with an account that has Premium. Spotify can take
  a few hours to notice a new subscription. The developer dashboard says so
  too: the app's page reads "Your application is blocked from accessing the
  Web API since you do not have a Spotify Premium subscription", and the
  **Web API** box can't be ticked when you create an app. A new app of the
  same account doesn't help: Spotify blocks them all. Without Premium,
  MixMusic still searches and plays YouTube Music, with no account at all.
- **Spotify refused (HTTP 403)**, without a word of why: check both of the
  above. Spotify can take a few minutes to let in an account just added.

Then choose **sign in to spotify** again, or **sign out** to stop using
Spotify.

MixMusic never sees your password. It keeps the token Spotify gives it in
your system's credential store: the Keychain on macOS, the Credential Manager
on Windows, the Secret Service (GNOME Keyring, KWallet…) on Linux. On Windows,
where the Credential Manager can't keep it, it is encrypted for your Windows
account in `spotify-token.dpapi`, in MixMusic's data folder. On Linux without
a Secret Service, it is in a file only your user can read,
`spotify-token.json` in the data folder; once a Secret Service is there, the
token moves into it and the file goes.

## How Spotify tracks play

Settings › Playback chooses:

- **Here, from Spotify** (the default on Windows). MixMusic plays the track
  itself, with Spotify's Web Playback SDK, at Spotify's own quality, and no
  Spotify app needs to be open: MixMusic shows among your Spotify devices
  while it plays. It needs Spotify Premium, and Windows, whose webview,
  WebView2, plays Spotify's protected audio: WebKitGTK on Linux does not,
  and macOS's is not known to, as Settings › Playback says there. The
  equalizer and crossfade leave Spotify's audio as it is, as on Spotify
  Connect: use a system-wide equalizer for it, since Spotify
  [notes](https://support.spotify.com/us/article/equalizer/) that its audio
  settings can't change while another device plays it. The visualizer and
  the plugins' analyser hear it, on Windows 10 build 20348 and later.
  Sign-ins made before MixMusic played Spotify itself do not allow it:
  Settings › Playback asks you to sign in again. When Spotify cannot play
  here, MixMusic plays the track from YouTube Music instead, and says why.
- **On Spotify Connect** (the default elsewhere). A Spotify device plays the track: your
  Spotify app on this computer, your phone, a speaker. It needs Spotify
  Premium, and a device with Spotify open. The devices button of the player
  bar chooses which device plays; by default, the active one. When no device
  can play, MixMusic plays the track from YouTube Music instead, and says so.
  On Windows, the visualizer and the plugins' analyser hear the Spotify app
  on this computer, as it plays, never recording it; other devices they
  can't hear, and the visualizer shows nothing then.
- **Here, from YouTube Music.** MixMusic finds the track's recording on YouTube
  Music and plays it in its own player, with the equalizer, crossfade and the
  visualizer. It works without Premium. When a match is wrong, choose **Find
  another YouTube Music match** in the track's menu.

## What Spotify allows apps

Apps in Spotify's development mode, as yours is, have limits:

- The app's owner needs Spotify Premium.
- Up to five accounts, added under **Users Management**, can use it.
- A search returns at most ten results of each kind.
- Apps see the tracks of the playlists you own or collaborate on, not of
  playlists others made. MixMusic tells you when a playlist is one of those.
- Spotify removed the artist top tracks endpoint: MixMusic finds an artist's
  popular tracks with a search instead.
- Requests are counted against a quota; when MixMusic hits it, Spotify asks it
  to wait, and MixMusic says so.
