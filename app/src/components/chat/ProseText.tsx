import { createMemo } from 'solid-js';
import { marked } from 'marked';
import DOMPurify from 'dompurify';

interface ProseTextProps {
  content: string;
}

export function ProseText(props: ProseTextProps) {
  const html = createMemo(() => {
    const rawHtml = marked.parse(props.content) as string;
    return DOMPurify.sanitize(rawHtml);
  });

  return (
    <div
      innerHTML={html()}
      class="[&_hr]:border-neu-800"
      style={{ "white-space": "pre-wrap" }}
    />
  );
}
