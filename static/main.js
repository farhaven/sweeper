let viewport = {
	width: 20,
	height:20
}

function handleMessage(socketMessage) {
	var message = JSON.parse(socketMessage.data);
	console.log("got a new message", message);

	// Update position display
	var locSpan = document.getElementById("location")
	locSpan.innerText = message.Score + " @ " + JSON.stringify(message.ViewPort.Position);

	for (y = 0; y < message.ViewPort.Data.length; y++) {
		for (x = 0; x < message.ViewPort.Data[y].length; x++) {
			var fe = document.getElementById("field-" + y + "-" + x);
			fe.classList.remove("flag");
			fe.classList.remove("boom");
			fe.classList.remove("dim");
			fe.classList.remove("mark");
			fe.classList.remove("number");
			fe.innerText = String.fromCharCode(message.ViewPort.Data[y][x]);
			switch (fe.innerText) {
				case "P":
					fe.classList.add("flag");
					break;
				case "X":
					fe.classList.add("boom");
					break;
				case "0":
					fe.classList.add("dim");
					break;
				case "?":
					fe.classList.add("mark");
					break;
				case " ":
					// no extra class
					break;
				default:
					fe.classList.add("number");
					break;
			}
		}
	}
}

function setup() {
	// Build playing field
	var field = document.getElementById("field");
	for (y = 0; y < viewport.width; y++) {
		var row = document.createElement("div");
		row.classList.add("fieldRow");
		for (x = 0; x < viewport.height; x++) {
			var elem = document.createElement("div");
			elem.id = "field-" + y + "-" + x;
			elem.classList.add("fieldElement");
			elem.dataset.x = x;
			elem.dataset.y = y;
			row.appendChild(elem);
		}
		field.appendChild(row);
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
		ws.addEventListener("message", handleMessage);
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


	var mode = "mark";

	var field = document.getElementById("field");
	field.addEventListener("click", event => {
		console.log("click", event);
		event.preventDefault();
		var x = parseInt(event.target.dataset.x);
		var y = parseInt(event.target.dataset.y);
		var request = {
			Kind: mode,
			X: x,
			Y: y
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