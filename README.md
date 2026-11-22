# WoWCensus Companion

A small app that sends your WoWCensus add-on data to [wowcensus.io](https://wowcensus.io), so the site's population numbers include what you've seen in game.

You play as normal. The app runs quietly in the background and shares new data each time you log out or `/reload`.

## Install

1. Download the latest version from the [Releases](https://github.com/wowcensus/companion/releases) page.
2. Run it. The first time you open it, it copies itself to its permanent location (`C:\WoWCensus` on Windows, `~/Applications` on macOS, `~/.local/opt` on Linux) and restarts from there. The app then finds your World of Warcraft folders on its own.
3. If it says the add-on isn't installed, click **Install add-on**. It puts the [WoWCensus add-on](https://github.com/wowcensus/addon) in the right folder for each game version you have. Restart WoW afterwards.
4. Click **Turn on uploads**.

You don't need an account, a password or a token.

## Using it

- Play WoW with the add-on enabled. Whenever you log out or type `/reload`, the app notices the new data and sends it.
- Closing the window keeps the app running in the system tray. To quit, right-click the tray icon and choose **Quit**.
- To start it automatically, turn on **Start when I log in to my computer** in Settings.

## What gets shared

**Shared:**
- what the WoWCensus add-on recorded: character names, realms, races, classes and levels it saw through `/who`. The first upload is the whole save; after that, each `/reload` or logout sends only what is new since the last upload.
- only if you tick them under **Other add-ons** in Settings: auction prices and your own sales from Auctionator, Journalator and TradeSkillMaster, and your Dungeon Journal progress. Other players' names, mail and chat text are removed on your computer before anything is sent.
- only if you tick **Combat logs**: what mobs did in your combat logs from any WoW edition (level, health, armor and attacks) for the bestiary on wowcensus.io. The app keeps only mob events and removes every player name and ID on your computer first. It needs Advanced Combat Logging on and `/combatlog` typed in game, and sends a log once WoW has stopped writing to it for 10 minutes.
- **Delete old combat logs** (off by default): WoW never deletes its combat logs, so they pile up. With this on, the app deletes a log once wowcensus.io has imported it and it is older than the cleanup window (14 days), only while WoW is closed. The newest log in each Logs folder is always kept. Leave it off if you also upload logs to other sites.

**Never shared:**
- your Battle.net account or login;
- your WoW account folder name;
- file paths on your computer;
- other players' names from your other add-ons;
- data from any other add-on you haven't ticked.

Uploads are off until you turn them on, and you can turn them off at any time in Settings.

## Something not working?

- **"No WoWCensus saves found yet"**: install the add-on, log in to a character with the add-on enabled, then `/reload` once. If WoW is installed somewhere unusual, add its folder in Settings.
- **Uploads failing**: the app retries on its own. The **Activity** tab shows what happened.
- **Add-on install fails**: close World of Warcraft and try again.

## Uninstall

Turn off **Start when I log in** in Settings, then quit the app from the tray. On Windows, delete `C:\WoWCensus` (or the earlier `%LocalAppData%\Programs\WoWCensus Companion` location). On macOS, delete `~/Applications/WoWCensus Companion.app`. On Linux, delete `~/.local/opt/wowcensus-companion/` and `~/.local/share/applications/wowcensus-companion.desktop`.

Settings and upload receipts are kept separately. To remove those too, delete the `WoWCensusCompanion` folder in `%AppData%` on Windows, `~/Library/Application Support` on macOS, or `~/.config` on Linux. The WoWCensus game add-on is separate and remains installed.
