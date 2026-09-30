// Package host is what a plugin that runs triggers needs, and a game never imports: the hosts
// that run triggers in a plugin's own pass, and the constructors act.Trigger builds them from.
//
// # Constructors
//
// [PairOf] reacts to every pair a host meets where one entity is on side a and the other on b
// ([SideOf] a tag, [AnySide] for either); [EachWith] reacts on every entity the host visits that
// carries a [State] ([StateOf] a component); [Every] on all of them. [Pair] and [Each] are the
// same, typed. act.Trigger picks among them by its filters, so a game names act and the
// plugin, never this package.
//
// # Hosts
//
// [PairHost] runs pair triggers: Bind its families to the host's queries once, then read what an
// entity carries (InChunk, At) as plugin.Marks and Dispatch, DispatchEitherWay or DispatchGrouped
// per pair or per observer. [EachHost] runs triggers over entities: Bind, then Run over each
// chunk walked with a function describing its i-th entity; triggers over one component share
// its column, and [Own] shares one the host reads itself. [ListHost] runs triggers of a moment
// of no entity — the clock's — once a pass. All refuse another payload's trigger with
// plugin.ErrUnhosted and a late one with plugin.ErrHostBuilt.
package host
