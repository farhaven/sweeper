var Admin = {
	request: async function(request) {
		 let resp = await fetch("/admin", {
			method: "POST",
			body: JSON.stringify(request),
			credentials: "same-origin",
			headers: {
				"Content-Type": "application/json",
			},
		});

		return await resp.json();
	},

	tableData: function(text, code) {
		let elem = document.createElement("td");
		if (!!code) {
			let code = document.createElement("code");
			code.innerText = text;
			elem.appendChild(code);
		} else {
			elem.innerText = text;
		}
		return elem;
	},

	tableUpdate: function(target) {
		let table = document.getElementById(target);
		let body = document.createElement("tbody");
		return {
			addRow: function(row) {
				body.appendChild(row);
			},
			done: function() {
				table.innerHTML = body.innerHTML;
			}
		};
	},

	updatePlayers: function() {
		console.log("updating list of players");
		let update = Admin.tableUpdate("players");
		function buildDeleteButton(player) {
			let btn = document.createElement("input");
			btn.value = "X";
			btn.type = "button";
			btn.addEventListener("click", event => {
				console.log("would delete player", player, "now");
			});
			console.log("btn", btn);
			let td = document.createElement("td");
			td.appendChild(btn);
			return td;
		};

		function addRow(idx, player) {
			let row = document.createElement("tr");
			row.appendChild(Admin.tableData(idx));
			row.appendChild(Admin.tableData(player.Id, true));
			row.appendChild(Admin.tableData(player.Name));
			row.appendChild(Admin.tableData(player.Score));
			row.appendChild(buildDeleteButton(player));
			update.addRow(row);
		};
		Admin.request({Request: "get-players"}).then(players => {
			console.log("got players", players);
			for (idx = 0; idx < players.length; idx++) {
				addRow(idx, players[idx]);
				console.log("player", players[idx]);
			}
			update.done();
		});
	},
	updateAdmins: function() {
		console.log("updating list of admins");
		let update = Admin.tableUpdate("admins");
		function addRow(idx, admin) {
			let row = document.createElement("tr");
			row.appendChild(Admin.tableData(idx));
			row.appendChild(Admin.tableData(admin.Id, true));
			row.appendChild(Admin.tableData(admin.Name));
			update.addRow(row);
		};
		Admin.request({Request: "get-admins"}).then(admins => {
			console.log("got admins", admins);
			for (idx = 0; idx < admins.length; idx++) {
				addRow(idx, admins[idx]);
			}
			update.done();
		});
	},

	setup: function() {
		console.log("admin setup called");
		Admin.updatePlayers();
		Admin.updateAdmins();
	},
};

window.addEventListener("load", Admin.setup, false);