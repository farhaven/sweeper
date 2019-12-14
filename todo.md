# TODO
## UI
### Browser
- add right click/long tap options:
	- place flag
	- place "unknown" marker
- support touchpad dragging to move viewport
- prettier display of currently active viewport location
- handle disconnect
	- reconnect websocket
	- then, move viewport

### Beamer
- build

## Backend
- handle more requests:
	- place flag
	- place 'unknown' marker
- only send viewport updates to overlapping viewports
	- also consider 30 units uncovering radius for flood fills

## Misc
- Player names
- Adjustable viewport size
- Score
	- Highscore list
- Handle "KABOOM" by ending game
	- announce player who triggered the kaboom
- Multiple minefields?
- Show player viewports
- Identify "hotspots"
	- by activity (clicks/second)?
- documentation?
- propaganda
