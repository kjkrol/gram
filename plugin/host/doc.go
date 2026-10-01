// Package host is what a plugin that runs rules needs, and a game never imports: the hosts
// that run rules in a plugin's own pass, and the constructors rule.On builds them from.
//
// # Constructors
//
// [PairOf] reacts to every pair a host meets where one entity is on side a and the other on b
// ([SideOf] a tag, [AnySide] for either); [EachWith] reacts on every entity the host visits that
// carries a [State] ([StateOf] a component); [Every] on all of them. [Pair] and [Each] are the
// same, typed. rule.On picks among them by its filters, so a game names rule and the
// plugin, never this package.
//
// # Hosts
//
// [PairHost] runs pair rules: Bind its families to the host's queries once, then read what an
// entity carries (InChunk, At) as plugin.Marks and Dispatch, DispatchEitherWay or DispatchGrouped
// per pair or per observer. [EachHost] runs rules over entities: Bind, then Run over each
// chunk walked with a function describing its i-th entity; rules over one component share
// its column, and [Own] shares one the host reads itself. [ListHost] runs rules of a moment of
// the world as a whole — the clock's — once a pass, walking no entities. All refuse another payload's rule with
// plugin.ErrUnhosted and a late one with plugin.ErrHostBuilt.
package host
