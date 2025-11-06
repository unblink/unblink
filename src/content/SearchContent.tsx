import SearchBar from "~/src/SearchBar";

export default function SearchContent() {
  return <div class="h-screen py-2 pr-2">
    <div class="flex items-center flex-col space-y-16 relative isolate overflow-auto py-2  bg-neu-900 h-full rounded-2xl border border-neu-800 ">
      <div class="relative z-40 mt-[40vh] ">
        <SearchBar variant="lg" />
      </div>
    </div>
  </div>
}