'use client'

import { ReactNode, useState } from "react";
import gaming_svg from "@/public/images/gaming.svg";
import creative_svg from "@/public/images/creative.svg";
import esports_svg from "@/public/images/esports.svg";
import irl_svg from "@/public/images/irl.svg";
import music_svg from "@/public/images/music.svg";
import amongus_img from "@/public/images/amongus.jpg";
import Image from "next/image";
import { cn } from "@/utils/cn";
import { ClassValue } from "clsx";
import Tag from "@/components/Tag";
import TagButton from "@/components/buttons/TagBtn";
import { Hover3DBox } from "@/components/Hover3DBox";
// import { Stream } from "@/models/Stream";
import { Tab, TabContent } from "@/components/Tab";
import { SearchInput } from "@/components/Input";
import { DefaultOption } from "@/components/Option";
import { Combobox, Option } from "@/components/ComboBox";
import { LuArrowDownWideNarrow, LuSparkles } from "react-icons/lu";

const BrowseItem = ({ title, icon }: { title: string; icon: any }) => {
  return (
    <div className="w-full relative bg-secondary rounded-md flex flex-row items-center justify-between px-4 py-2 cursor-pointer hover:bg-primary ease-linear duration-100">
      <h1 className="text-white font-bold text-2xl">{title}</h1>
      <div className="w-[80px]"></div>
      <Image
        src={icon}
        width={80}
        height={80}
        alt="Icon"
        className="absolute end-2"
      />
    </div>
  );
};

const CategoryContentView = ({
  title,
  viewers,
  tags,
}: {
  title: string;
  viewers: number;
  tags: string[];
}) => {
  return (
    <div className="flex-1 flex-col space-y-1">
      <span className="text-sm hover:text-primary cursor-pointer font-semibold">
        {title}
      </span>
      <div className="text-sm text-secondaryWord cursor-pointer">
        {viewers} viewers
      </div>
      <div className="flex flex-row gap-2 justify-self-end">
        {tags.map((tag, idx) => {
          return <TagButton key={idx} content={tag} />;
        })}
      </div>
    </div>
  );
};

const CategoryView = ({
  className,
  title,
  viewers,
  tags,
}: {
  className?: ClassValue;
  title: string;
  viewers: number;
  tags: string[];
}) => {
  return (
    <div className="flex flex-col">
      <Hover3DBox imageSrc={amongus_img} className="h-[260px]" />
      <CategoryContentView title={title} tags={tags} viewers={viewers} />
    </div>
  );
};

const CategoryListView = ({
  className,
  limitView,
  streams,
}: {
  className?: ClassValue;
  limitView: number;
  streams: any[];
}) => {
  const streamingData = streams.slice(0, limitView);
  return (
    <div
      className={cn(
        "w-full grid xl:grid-cols-6 lg:grid-cols-4 md:grid-cols-3 max-md:grid-cols-2 max-sm:grid-cols-1 gap-6",
        className
      )}
    >
      {streamingData.map((streaming, idx) => {
        return (
          <CategoryView
            key={idx}
            title={streaming.title}
            tags={streaming.tags}
            viewers={120}
          />
        );
      })}
    </div>
  );
};

export default function BrowsePage() {
  const browses: { title: string; icon: ReactNode }[] = [
    {
      title: "Games",
      icon: gaming_svg,
    },
    {
      title: "IRL",
      icon: irl_svg,
    },
    {
      title: "Music",
      icon: music_svg,
    },
    {
      title: "Esports",
      icon: esports_svg,
    },
    {
      title: "Creative",
      icon: creative_svg,
    },
  ];

  const [selectedTab, setSelectedTab] = useState("Categories");
  const [sortFilter, setSortFilter] = useState("Recommended For You");
  const [tagFilter, setTagFilter] = useState<string>("");
  const handleDeleteTag = () => {
    setTagFilter("");
  };

  return (
    <div className="w-full flex flex-col p-8 h-full overflow-y-scroll">
      <h1 className="text-5xl font-bold">Browse</h1>
      <div className="mt-6 w-full lg:flex lg:flex-row max-lg:grid max-lg:grid-cols-3 max-lg:gap-8 max-md:grid-cols-2 max-sm:grid-cols-1 items-center justify-start gap-2">
        {browses.map((browse, idx) => (
          <BrowseItem key={idx} title={browse.title} icon={browse.icon} />
        ))}
      </div>
      <div className="flex flex-row items-center gap-6 mt-6">
        <Tab
          content="Categories"
          className="text-lg font-semibold"
          selectedTab={selectedTab}
          setSelectedTab={setSelectedTab}
        />
        <Tab
          content="Live channels"
          className="text-lg font-semibold"
          selectedTab={selectedTab}
          setSelectedTab={setSelectedTab}
        />
      </div>

      <TabContent
        contentFor="Categories"
        selectedTab={selectedTab}
        content={
          <div>
            <div className="flex sm:flex-row max-sm:flex-col max-sm:gap-2 max-sm:items-start sm:justify-between items-center mt-8">
              <div className="flex flex-row items-center gap-4">
                <SearchInput
                  id="search-input"
                  placeholder="Search Category Tags"
                  className="text-sm w-[250px] max-sm:w-full pr-2"
                  popoverPosition="bottom-start"
                  popoverContent={
                    <div className="flex flex-col">
                      {["Cat1, Cat2, Cat3"].map((category, idx) => {
                        return (
                          <DefaultOption
                            key={idx}
                            content={<span>{category}</span>}
                            onClick={() => setTagFilter(category)}
                          />
                        );
                      })}
                    </div>
                  }
                />

                <Tag
                  className={cn(tagFilter === "" ? "hidden" : "")}
                  onDelete={handleDeleteTag}
                >
                  {tagFilter}
                </Tag>
              </div>
              <div className="flex flex-row items-center gap-4">
                <span className="font-semibold text-sm text-black whitespace-nowrap">
                  Sort by
                </span>
                <Combobox
                  selectedOption={sortFilter}
                  className="text-sm"
                  popoverPosition="bottom-end"
                  popoverContent={
                    <div className="w-full flex flex-col items-center">
                      <Option
                        icon={<LuSparkles />}
                        content="Recommended For You"
                        className="text-sm"
                        selectedOption={sortFilter}
                        setSelectedOption={setSortFilter}
                      />
                      <Option
                        icon={<LuArrowDownWideNarrow />}
                        content="Viewers (High to Low)"
                        className="text-sm"
                        selectedOption={sortFilter}
                        setSelectedOption={setSortFilter}
                      />
                    </div>
                  }
                />
              </div>
            </div>

            <CategoryListView
              limitView={12}
              streams={[]}
              className="mt-6"
            />
          </div>
        }
      />
    </div>
  );
}
