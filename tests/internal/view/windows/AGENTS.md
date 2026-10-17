# Windows UI tests

`menu_test.go` runs portable control tests for clicks, keyboard input, narrow windows, text scaling, and saved branding. `caption_test.go` requires `desktop && windows` and checks real caption-button pixels.

packages.json maps both back to view, retaining private state, image fixtures, and desktop TestMain. Directory grouping changes neither platform conditions nor the working directory. Do not create a separate test event loop. Update the parent map and product view/windows map.
