// [INPUT]: The fixed application identity.
// [OUTPUT]: MyGo's internal name shared by startup and platform Shell identities.
// [POS]: Stable identity independent of the user-visible name.
// [PROTOCOL]: Update the owning AGENTS.md and behavior tests under tests/.
package appearance

// InternalName fixes MyGo identity, the single-instance lock, and browser data paths; it must not follow the display name.
const InternalName = "PageDock"
