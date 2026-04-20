"use client";

import { EditorContent, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import { useEffect } from "react";

interface EditorProps {
  initialText?: string;
  onChange: (plainText: string) => void;
}

export function Editor({ initialText = "", onChange }: EditorProps) {
  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: { levels: [1, 2, 3] },
      }),
      Placeholder.configure({
        placeholder:
          "Start writing. Try listing your expenses, your to-dos, or just dump your thoughts…",
      }),
    ],
    content: initialText
      ? { type: "doc", content: [{ type: "paragraph", content: [{ type: "text", text: initialText }] }] }
      : "",
    editorProps: {
      attributes: {
        class: "prose prose-invert max-w-none h-full p-6 focus:outline-none",
      },
    },
    immediatelyRender: false, // avoid SSR hydration mismatch warnings in Next.js App Router
  });

  useEffect(() => {
    if (!editor) return;
    const handler = () => onChange(editor.getText());
    editor.on("update", handler);
    return () => {
      editor.off("update", handler);
    };
  }, [editor, onChange]);

  return (
    <div className="h-full overflow-y-auto">
      <EditorContent editor={editor} className="h-full" />
    </div>
  );
}
