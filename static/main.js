var Sweeper = {
	Viewport: {
		width:  20,
		height: 20
	},

	Field: {
		ctx: null,
		width: 1200,
		height: 1200,
		xscale: null,
		yscale: null,
		fontSz: null,
	},

	updateScale: function() {
		let padding = 10;

		let canvas = document.getElementById("field");
		let container = document.getElementById("container");
		let width = container.clientWidth - padding;

		// Large screen
		var maxFieldWidth = width * 0.66; /* 2/3rd for field */
		var scale = (Math.min(maxFieldWidth, window.innerHeight) - (padding * 2)) / Sweeper.Field.width;

		// "Small" screen
		if (width < 1020) {
			scale = (Math.min(width, window.innerHeight) - (padding * 2)) / Sweeper.Field.width;
		}

		Sweeper.Field.xscale = scale;
		Sweeper.Field.yscale = scale;

		// Use CSS to scale the canvas
		canvas.style.width = parseInt(Sweeper.Field.width * Sweeper.Field.xscale) + "px";
		canvas.style.height = parseInt(Sweeper.Field.height * Sweeper.Field.yscale) + "px";
		canvas.style["margin-left"] = padding + "px";
	},

	updateHighscores: function(scores) {
		let tbody = document.createElement("tbody");
		for (idx = 0; idx < scores.length; idx++) {
			let row = document.createElement("tr");

			let place = document.createElement("td");
			place.innerText = idx + 1;
			row.appendChild(place);

			let name = document.createElement("td");
			name.innerText = scores[idx].Name;
			name.classList.add("fixed-width");
			row.appendChild(name);

			let score = document.createElement("td");
			score.innerText = scores[idx].Score;
			row.appendChild(score);

			tbody.appendChild(row);
		}
		let highscoreTable = document.getElementById("scoredata");
		highscoreTable.innerHTML = tbody.innerHTML;
	},

	handleMessage: function(socketMessage) {
		var message = JSON.parse(socketMessage.data);

		// Update position display
		var locSpan = document.getElementById("location");
		locSpan.innerText = message.Score + " @ " + JSON.stringify(message.ViewPort.Position);

		// Update player name
		var playerName = document.getElementById("player-name");
		playerName.value = message.Name;

		Sweeper.clearField();

		// Update field display
		for (y = 0; y < message.ViewPort.Data.length; y++) {
			for (x = 0; x < message.ViewPort.Data[y].length; x++) {
				var txt = String.fromCharCode(message.ViewPort.Data[y][x]);
				var textStyle = null;
				var fillStyle = null;

				switch (txt) {
					case "P":
						textStyle = "darkred";
						break;
					case "X":
						textStyle = "red";
						fillStyle = "black";
						break;
					case "0":
						textStyle = "#bbb";
						break;
					case "?":
						textStyle = "darkblue";
						break;
					case " ":
						// no extra stuff
						txt = null;
						break;
					default:
						textStyle = "#666";
						break;
				}

				Sweeper.drawFieldElement(x, y, txt, textStyle, fillStyle);
			}
		}

		Sweeper.updateHighscores(message.Highscores);
	},

	clearField: function() {
		Sweeper.Field.ctx.clearRect(0, 0, Sweeper.Field.width, Sweeper.Field.height);
	},

	drawFieldElement: function (col, row, text, textStyle, fillStyle) {
		let numCols = Sweeper.Field.width / Sweeper.Viewport.width;
		let numRows = Sweeper.Field.height / Sweeper.Viewport.height;

		Sweeper.Field.ctx.strokeStyle = "1px solid black";
		Sweeper.Field.ctx.strokeRect(col * numCols, row * numRows, numCols, numRows);

		if ((fillStyle != undefined) && (fillStyle != null)) {
			Sweeper.Field.ctx.fillStyle = fillStyle;
			Sweeper.Field.ctx.fillRect(col * numCols, row * numRows, numCols, numRows);
		}

		if ((text != undefined) && (text != null)) {
			Sweeper.Field.ctx.font = "bold " + Sweeper.Field.fontSz + "px Monospace";
			var metrics = Sweeper.Field.ctx.measureText(text);
			Sweeper.Field.ctx.fillStyle = textStyle;
			Sweeper.Field.ctx.fillText(text, (col * numCols) + metrics.width * 0.75, (row * numRows) + metrics.width * 2);
		}
	},

	setup: function() {
		// Build playing field
		var canvas = document.getElementById("field");
		canvas.width = Sweeper.Field.width;
		canvas.height = Sweeper.Field.height;

		Sweeper.Field.fontSz = (Sweeper.Field.width / Sweeper.Viewport.width) / 1.5;
		Sweeper.updateScale();
		window.addEventListener("resize", Sweeper.updateScale, false);

		Sweeper.Field.ctx = canvas.getContext('2d');
		Sweeper.clearField();

		for (col = 0; col < Sweeper.Viewport.width; col++) {
			for (row = 0; row < Sweeper.Viewport.height; row++) {
				Sweeper.drawFieldElement(col, row);
			}
		}

		// TODO: Add handlers for other events?
		var protocol = "ws"
		if (document.location.protocol == "https:") {
			protocol = "wss"
		}
		var path = "/ws"
		if (document.location.pathname == "/") {
			path = "ws"
		}

		let socketURL = protocol + "://" + document.location.host + document.location.pathname + path;

		var ws = null;
		var connectSocket = function() {
			try {
				ws = new WebSocket(socketURL);
			} catch (e) {
				console.log("can't create websocket", e);
				return;
			}
			ws.addEventListener("message", Sweeper.handleMessage);
			ws.addEventListener("close", event => {
				console.log("reconnecting", event);
				connectSocket();
			});
			ws.addEventListener("error", event => {
				console.log(event);
			});
		};
		connectSocket();

		// Add event handlers for inputs
		document.addEventListener("keydown", event => {
			event.preventDefault();

			var request = {
				Kind: "move",
				X: 0,
				Y: 0
			}

			switch (event.key) {
				case "ArrowLeft":
					request.X = -1;
					break;
				case "ArrowRight":
					request.X = 1;
					break;
				case "ArrowUp":
					request.Y = -1;
					break;
				case "ArrowDown":
					request.Y += 1;
					break;
				default:
					return;
			}

			ws.send(JSON.stringify(request));
		})

		var field = document.getElementById("field");

		function mapEventToField(event) {
			let xscale = (Sweeper.Field.width / Sweeper.Viewport.width) * Sweeper.Field.xscale;
			let yscale = (Sweeper.Field.height / Sweeper.Viewport.height) * Sweeper.Field.yscale;

			let r = field.getBoundingClientRect();
			let x = parseInt((event.clientX - r.left) / xscale);
			let y = parseInt((event.clientY - r.top) / yscale);

			return {X: x, Y: y};
		}

		function handleClick(event) {
			event.preventDefault();
			if (inTouch) {
				inTouch = false;
				return;
			}
			clearTouchTimeouts();

			var request = mapEventToField(event);

			if ((new Date()) - touchTime > 1000) {
				request.Kind = "uncover";
			} else {
				switch (event.button) {
					case 0:
						request.Kind = "mark"
						break;
					case 2:
						request.Kind = "uncover"
						break;
					default:
						console.log("unexpected buttons:", event.buttons, "defaulting to mark");
						request.Kind = "mark"
						break;
				}
			}
			ws.send(JSON.stringify(request));
		}

		var touchTime = null;
		var touchTimeouts = [];
		function registerTouchTimeout() {
			touchTime = new Date();
			touchTimeouts.push(setTimeout(function() {
				var loc = document.getElementById("location");
				loc.classList.add("notify");
				setTimeout(function() {
					loc.classList.remove("notify");
				}, 400);
			}, 1000));
		}
		function clearTouchTimeouts() {
			for (idx = 0; idx < touchTimeouts.length; idx++) {
				clearTimeout(touchTimeouts[idx]);
			}
			touchTimeouts = [];
		}

		field.addEventListener("contextmenu", event => event.preventDefault());
		field.addEventListener("pointerdown", event => {
			event.preventDefault();
			registerTouchTimeout();
		});
		field.addEventListener("pointerup", handleClick);

		// Touch event handling
		var touchX = null;
		var touchY = null;
		var inTouch = false;

		field.addEventListener("touchstart", event => {
			event.preventDefault();
			touchX = event.touches[0].clientX;
			touchY = event.touches[0].clientY;
			inTouch = true;
			registerTouchTimeout();
		});

		field.addEventListener("touchmove", event => {
			event.preventDefault();
			var x = event.touches[0].clientX;
			var y = event.touches[0].clientY;

			// Sometimes, jittery fingers cause small movement events, even if the finger didn't really move.
			if (Math.sqrt(Math.pow(x - touchX, 2) + Math.pow(y - touchY, 2)) < 15) {
				return;
			}
			clearTouchTimeouts();
		});

		field.addEventListener("touchend", event => {
			event.preventDefault();
			clearTouchTimeouts();

			let xscale = (Sweeper.Field.width / Sweeper.Viewport.width) * Sweeper.Field.xscale;
			let yscale = (Sweeper.Field.height / Sweeper.Viewport.height) * Sweeper.Field.yscale;

			let touch = event.changedTouches[0];
			let deltaX = parseInt((touchX - touch.clientX) / xscale);
			let deltaY = parseInt((touchY - touch.clientY) / yscale);

			var request = null;
			if ((deltaX == 0) && (deltaY == 0)) {
				var request = mapEventToField(touch);
				let timeDelta = (new Date()) - touchTime;
				if (timeDelta > 1000) {
					// Pressed for more than 2 seconds
					request.Kind = "uncover"
				} else {
					request.Kind = "mark"
				}
			} else {
				request = {
					Kind: "move",
					X: deltaX,
					Y: deltaY
				}
			}
			ws.send(JSON.stringify(request));
		});

		// Wire up side bar
		function sidebar(showWhatsThis, showHighscores) {
			let whatsthis = document.getElementById("whatsthis");
			let highscores = document.getElementById("highscores");
			whatsthis.hidden = !showWhatsThis;
			highscores.hidden = !showHighscores;

			let selectWhatsThis = document.getElementById("select-whatsthis");
			let selectHighscores = document.getElementById("select-highscores");

			if (showWhatsThis) {
				selectWhatsThis.classList.add("pure-menu-selected");
				selectHighscores.classList.remove("pure-menu-selected");
			}
			if (showHighscores) {
				selectWhatsThis.classList.remove("pure-menu-selected");
				selectHighscores.classList.add("pure-menu-selected");
			}
		}

		document.getElementById("select-whatsthis").addEventListener("click", event => {
			event.preventDefault();
			sidebar(true, false);
		});
		document.getElementById("select-highscores").addEventListener("click", event => {
			event.preventDefault();
			sidebar(false, true);
		})

		// Highscore name entry
		let playerName = document.getElementById("player-name")
		playerName.addEventListener("change", event => {
			let request = {
				Kind: "update-name",
				Name: playerName.value,
			};
			ws.send(JSON.stringify(request));
		});
	}
};

window.addEventListener("load", Sweeper.setup, false);




