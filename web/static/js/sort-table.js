// Client-side column-header sort for the accounts table. Rows carry a
// data-sort-value on each <td> (see internal/templates/accounts/row.templ);
// clicking a <th data-sort-key> (see list.templ's sortableHeader) reorders
// <tbody id="account-rows"> in place. Sort state lives only in the DOM (a
// data-sort-dir attribute on the clicked <th>) and is not persisted — a
// page reload always goes back to the server's default (soonest-expiring
// first) order.
(function () {
	document.addEventListener("click", function (e) {
		var th = e.target.closest("th[data-sort-key]");
		if (!th) return;

		var table = th.closest("table");
		var tbody = table.querySelector("tbody");
		var headers = Array.from(th.parentElement.children);
		var index = headers.indexOf(th);

		var dir = th.dataset.sortDir === "asc" ? "desc" : "asc";
		headers.forEach(function (cell) {
			delete cell.dataset.sortDir;
			var arrow = cell.querySelector("[data-sort-arrow]");
			if (arrow) arrow.textContent = "";
		});
		th.dataset.sortDir = dir;
		var arrow = th.querySelector("[data-sort-arrow]");
		if (arrow) arrow.textContent = dir === "asc" ? "▲" : "▼";

		var sign = dir === "asc" ? 1 : -1;
		var isNumeric = th.dataset.sortType === "number";
		var rows = Array.from(tbody.children);
		rows.sort(function (a, b) {
			var av = (a.children[index] && a.children[index].dataset.sortValue) || "";
			var bv = (b.children[index] && b.children[index].dataset.sortValue) || "";
			if (isNumeric) return (parseFloat(av) - parseFloat(bv)) * sign;
			return av.localeCompare(bv) * sign;
		});
		rows.forEach(function (row) {
			tbody.appendChild(row);
		});
	});
})();
