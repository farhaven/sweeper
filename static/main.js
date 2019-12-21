var Sweeper = {
	Viewport: {
		width:  20,
		height: 20
	},

	Field: {
		ctx: null,
		width: 500,
		height: 500,
		xscale: 1.25,
		yscale: 1.25,
		fontSz: 14,
	},

	handleMessage: function(socketMessage) {
		var message = JSON.parse(socketMessage.data);
		console.log("handling message", message);

		// Update position display
		var locSpan = document.getElementById("location")
		locSpan.innerText = message.Score + " @ " + JSON.stringify(message.ViewPort.Position);

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
	},

	clearField: function() {
		Sweeper.Field.ctx.clearRect(0, 0, Sweeper.Field.width, Sweeper.Field.height);
	},

	drawFieldElement: function (col, row, text, textStyle, fillStyle) {
		let xscale = Sweeper.Field.width / Sweeper.Viewport.width;
		let yscale = Sweeper.Field.height / Sweeper.Viewport.height;

		Sweeper.Field.ctx.strokeStyle = "1px solid black";
		Sweeper.Field.ctx.strokeRect(col * xscale, row * yscale, xscale, yscale);

		if ((fillStyle != undefined) && (fillStyle != null)) {
			Sweeper.Field.ctx.fillStyle = fillStyle;
			Sweeper.Field.ctx.fillRect(col * xscale, row * yscale, xscale, yscale);
		}

		if ((text != undefined) && (text != null)) {
			Sweeper.Field.ctx.font = "bold " + Sweeper.Field.fontSz + "px Monospace";
			Sweeper.Field.ctx.fillStyle = textStyle;
			Sweeper.Field.ctx.fillText(text, (col * xscale) + Sweeper.Field.fontSz / 2, (row * yscale) + Sweeper.Field.fontSz * 1.25);
		}
	},

	setup: function() {
		// Build playing field
		var canvas = document.getElementById("field");
		canvas.width = Sweeper.Field.width;
		canvas.height = Sweeper.Field.height;

		// Use CSS to scale the canvas
		canvas.style.width = parseInt(Sweeper.Field.width * Sweeper.Field.xscale) + "px";
		canvas.style.height = parseInt(Sweeper.Field.height * Sweeper.Field.yscale) + "px";

		Sweeper.Field.ctx = canvas.getContext('2d');
		Sweeper.clearField();

		for (col = 0; col < Sweeper.Viewport.width; col++) {
			for (row = 0; row < Sweeper.Viewport.height; row++) {
				Sweeper.drawFieldElement(col, row);
			}
		}

		// TODO: Add handlers for other events?
		var protocol = "ws"
		if (document.location.protocol == "https") {
			protocol = "wss"
		}
		var path = "/ws"
		if (document.location.pathname == "/") {
			path = "ws"
		}

		let socketURL = protocol + "://" + document.location.host + document.location.pathname + path
		console.log("socketurl", socketURL);

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
					console.log("unhandled key event", event);
					return;
			}

			console.log("sending request", request);

			ws.send(JSON.stringify(request));
		})

		var field = document.getElementById("field");

		// Mode switcher
		var mode = "mark";

		field.addEventListener("click", event => {
			console.log("click", event);
			event.preventDefault();

			let xscale = (Sweeper.Field.width / Sweeper.Viewport.width) * Sweeper.Field.xscale;
			let yscale = (Sweeper.Field.height / Sweeper.Viewport.height) * Sweeper.Field.yscale;

			var x = parseInt((event.clientX - event.target.offsetLeft) / xscale);
			var y = parseInt((event.clientY - event.target.offsetTop) / yscale);

			console.log("click", x, y);

			var request = {
				Kind: mode,
				X: x,
				Y: y
			}
			ws.send(JSON.stringify(request));
		});

		var touchX = null;
		var touchY = null;

		field.addEventListener("touchstart", event => {
			event.preventDefault();
			console.log("touchstart", event.touches[0]);
			touchX = event.touches[0].clientX;
			touchY = event.touches[0].clientY;
		});

		field.addEventListener("touchmove", event => {
			event.preventDefault();
		});

		field.addEventListener("touchend", event => {
			event.preventDefault();
			console.log("touchend", event.changedTouches[0]);

			let xscale = (Sweeper.Field.width / Sweeper.Viewport.width) * Sweeper.Field.xscale;
			let yscale = (Sweeper.Field.height / Sweeper.Viewport.height) * Sweeper.Field.yscale;

			let touch = event.changedTouches[0];
			let deltaX = parseInt((touchX - touch.clientX) / xscale);
			let deltaY = parseInt((touchY - touch.clientY) / yscale);

			console.log(deltaX, deltaY);

			var request = null;
			if ((deltaX == 0) && (deltaY == 0)) {
				let x = parseInt((touch.clientX - touch.target.offsetLeft) / xscale);
				let y = parseInt((touch.clientY - touch.target.offsetTop) / yscale);

				request = {
					Kind: mode,
					X: x,
					Y: y
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

		var modeMark = document.getElementById("mode-mark");
		var modeUncover = document.getElementById("mode-uncover");

		modeMark.addEventListener("click", event => {
			mode = "mark";
			modeUncover.classList.remove("active");
			modeMark.classList.add("active");
		});

		modeUncover.addEventListener("click", event => {
			mode = "uncover";
			modeUncover.classList.add("active");
			modeMark.classList.remove("active");
		});
	}
};

window.addEventListener("load", Sweeper.setup, false);




