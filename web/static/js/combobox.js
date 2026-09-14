// Minimal combobox behavior: an <input> that behaves like a dropdown (click
// it, or its chevron, to see every suggestion) while still accepting free
// text for a brand-new value. No build step, no dependency — just event
// delegation on `document`, so it works for any number of instances on the
// page, including ones HTMX swaps in later without any extra wiring.
//
// Markup contract (see internal/templates/accounts/form.templ):
//   <div data-combobox class="relative">
//     <input data-combobox-input ...>
//     <button type="button" data-combobox-toggle>...</button>
//     <ul data-combobox-list class="hidden ...">
//       <li data-combobox-option>Suggestion</li>
//     </ul>
//   </div>
(function () {
	function comboboxOf(el) {
		return el.closest("[data-combobox]");
	}

	function listOf(box) {
		return box.querySelector("[data-combobox-list]");
	}

	function closeAll() {
		document.querySelectorAll("[data-combobox-list]").forEach(function (list) {
			list.classList.add("hidden");
		});
	}

	// Opening always shows every suggestion, regardless of what's currently
	// typed — narrowing only happens once the user actually types more, so a
	// prefilled value (e.g. "Joint") never hides the rest of the list.
	function open(box) {
		closeAll();
		filter(box, "");
		listOf(box).classList.remove("hidden");
	}

	function filter(box, query) {
		var q = query.trim().toLowerCase();
		box.querySelectorAll("[data-combobox-option]").forEach(function (opt) {
			var match = q === "" || opt.textContent.toLowerCase().indexOf(q) !== -1;
			opt.classList.toggle("hidden", !match);
		});
	}

	document.addEventListener("focusin", function (e) {
		var input = e.target.closest("[data-combobox-input]");
		if (input) open(comboboxOf(input));
	});

	document.addEventListener("input", function (e) {
		var input = e.target.closest("[data-combobox-input]");
		if (!input) return;
		var box = comboboxOf(input);
		if (!listOf(box).classList.contains("hidden")) filter(box, input.value);
	});

	document.addEventListener("keydown", function (e) {
		if (e.key === "Escape") closeAll();
	});

	document.addEventListener("click", function (e) {
		var toggle = e.target.closest("[data-combobox-toggle]");
		if (toggle) {
			var box = comboboxOf(toggle);
			var wasHidden = listOf(box).classList.contains("hidden");
			closeAll();
			if (wasHidden) {
				open(box);
				box.querySelector("[data-combobox-input]").focus();
			}
			return;
		}

		var option = e.target.closest("[data-combobox-option]");
		if (option) {
			var input = comboboxOf(option).querySelector("[data-combobox-input]");
			input.value = option.textContent;
			closeAll();
			input.focus();
			return;
		}

		if (!comboboxOf(e.target)) closeAll();
	});
})();
