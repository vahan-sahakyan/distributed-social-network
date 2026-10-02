import { Link } from 'react-router-dom'

// same pattern search-service extracts hashtags with
const HASHTAG = /(#[\p{L}\p{N}_]+)/u

// RichText renders text with each #hashtag linking to its search.
export function RichText({ text = '' }) {
  return text.split(HASHTAG).map((part, i) =>
    i % 2 === 1 ? (
      <Link
        key={i}
        to={`/search?q=${encodeURIComponent(part.toLowerCase())}`}
        onClick={e => e.stopPropagation()}
        className="text-accent hover:underline"
      >
        {part}
      </Link>
    ) : (
      part
    ),
  )
}

// Highlight renders an Elasticsearch highlight, where matches are wrapped in
// <em></em>. Post text is user input and the highlighter doesn't escape it, so the
// string is split on the markers and rendered as text, never as HTML.
export function Highlight({ html = '' }) {
  return html.split(/(<em>.*?<\/em>)/s).map((part, i) =>
    part.startsWith('<em>') ? (
      <mark key={i} className="bg-accent/25 text-text rounded px-0.5">
        <RichText text={part.slice(4, -5)} />
      </mark>
    ) : (
      <RichText key={i} text={part} />
    ),
  )
}
